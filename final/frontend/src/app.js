document.addEventListener('DOMContentLoaded', function() {
    const videoPlayer = document.getElementById('videoPlayer');
    const streamPath = document.getElementById('streamPath');
    const playButton = document.getElementById('playButton');
    const streamStatus = document.getElementById('streamStatus');
    const streamList = document.getElementById('streamList');
    
    // Check if HLS.js is supported
    const isHlsSupported = Hls.isSupported();
    let hls;
    
    // Fetch available streams
    function fetchStreams() {
        fetch('/api/status')
            .then(response => response.json())
            .then(data => {
                streamList.innerHTML = '';
                
                if (Object.keys(data).length === 0) {
                    streamList.innerHTML = '<li>No active streams</li>';
                    return;
                }
                
                for (const [path, status] of Object.entries(data)) {
                    const li = document.createElement('li');
                    const active = status.isActive ? '🟢 ' : '🔴 ';
                    li.textContent = active + path;
                    li.dataset.path = path;
                    
                    // Add information about HLS availability
                    if (status.transcodingStatus) {
                        const statusBadge = document.createElement('span');
                        statusBadge.className = 'status-badge ' + status.transcodingStatus.toLowerCase();
                        statusBadge.textContent = status.transcodingStatus;
                        li.appendChild(document.createElement('br'));
                        li.appendChild(statusBadge);
                    }
                    
                    li.addEventListener('click', () => {
                        streamPath.value = path;
                        playStream(path);
                        updateStreamInfo(status);
                    });
                    streamList.appendChild(li);
                }
            })
            .catch(error => {
                console.error('Error fetching streams:', error);
                streamList.innerHTML = '<li>Error loading streams</li>';
            });
    }
    
    // Play HLS stream
    function playStream(path) {
        if (!path) {
            alert('Please enter a stream path');
            return;
        }
        
        const hlsUrl = `/hls/${path}/index.m3u8`;
        console.log(`Attempting to play: ${hlsUrl}`);
        
        if (isHlsSupported) {
            if (hls) {
                hls.destroy();
            }
            
            hls = new Hls({
                debug: false,
                // Low-Latency HLS Configuration
                lowLatencyMode: true,
                liveSyncDuration: 2,
                liveMaxLatencyDuration: 5,
                liveDurationInfinity: true,
                highBufferWatchdogPeriod: 1,
                // Tune these parameters for optimal low latency
                maxBufferLength: 4,
                maxMaxBufferLength: 6,
                maxBufferSize: 2 * 1000 * 1000, // 2MB
                maxBufferHole: 0.1,
                // Standard configurations
                manifestLoadingTimeOut: 10000,
                manifestLoadingMaxRetry: 4,
                manifestLoadingRetryDelay: 500,
                levelLoadingTimeOut: 10000,
                levelLoadingMaxRetry: 4,
                levelLoadingRetryDelay: 500,
                fragLoadingTimeOut: 10000,
                fragLoadingMaxRetry: 6,
                fragLoadingRetryDelay: 500
            });
            
            hls.loadSource(hlsUrl);
            hls.attachMedia(videoPlayer);
            hls.on(Hls.Events.MANIFEST_PARSED, function() {
                videoPlayer.play()
                    .catch(e => {
                        console.error('Error playing video:', e);
                        // Try to manually trigger HLS transcoding if playback fails
                        triggerTranscoding(path);
                    });
            });
            
            hls.on(Hls.Events.ERROR, function(event, data) {
                if (data.fatal) {
                    switch(data.type) {
                        case Hls.ErrorTypes.NETWORK_ERROR:
                            console.log('Network error', data);
                            hls.startLoad();
                            // Try to manually trigger HLS transcoding if network error occurs
                            triggerTranscoding(path);
                            break;
                        case Hls.ErrorTypes.MEDIA_ERROR:
                            console.log('Media error', data);
                            hls.recoverMediaError();
                            break;
                        default:
                            console.error('Fatal error:', data);
                            hls.destroy();
                            break;
                    }
                }
            });
        } else if (videoPlayer.canPlayType('application/vnd.apple.mpegurl')) {
            // For Safari
            videoPlayer.src = hlsUrl;
            videoPlayer.addEventListener('loadedmetadata', function() {
                videoPlayer.play()
                    .catch(e => {
                        console.error('Error playing video:', e);
                        // Try to manually trigger HLS transcoding if playback fails
                        triggerTranscoding(path);
                    });
            });
        } else {
            alert('HLS is not supported in your browser');
        }
        
        // Fetch stream status
        fetch(`/api/status?path=${path}`)
            .then(response => response.json())
            .then(data => {
                updateStreamInfo(data);
            })
            .catch(error => {
                console.error('Error fetching stream status:', error);
                streamStatus.innerHTML = 'Error loading stream information';
            });
    }
    
    // Manually trigger transcoding
    function triggerTranscoding(path) {
        console.log(`Manually triggering transcoding for: ${path}`);
        fetch(`/api/transcode?path=${path}`)
            .then(response => response.json())
            .then(data => {
                console.log('Transcoding response:', data);
                // Wait a moment and then try to play again
                setTimeout(() => {
                    playStream(path);
                }, 5000);
            })
            .catch(error => {
                console.error('Error triggering transcoding:', error);
            });
    }
    
    // Update stream information display
    function updateStreamInfo(status) {
        if (!status) {
            streamStatus.innerHTML = 'Stream information not available';
            return;
        }
        
        let html = `
            <div><strong>Path:</strong> ${status.streamPath}</div>
            <div><strong>Status:</strong> ${status.isActive ? 'Active' : 'Inactive'}</div>
            <div><strong>Transcoding:</strong> <span class="status-badge ${status.transcodingStatus?.toLowerCase() || 'unknown'}">${status.transcodingStatus || 'Unknown'}</span></div>
            <div><strong>HLS Available:</strong> ${status.hlsAvailable ? '✅ Yes' : '❌ No'}</div>
        `;
        
        if (status.startedAt) {
            html += `<div><strong>Started:</strong> ${new Date(status.startedAt).toLocaleString()}</div>`;
        }
        
        if (status.viewerCount !== undefined) {
            html += `<div><strong>Viewers:</strong> ${status.viewerCount}</div>`;
        }
        
        if (status.videoInfo && status.videoInfo.width) {
            html += `
                <div><strong>Resolution:</strong> ${status.videoInfo.width}x${status.videoInfo.height}</div>
                <div><strong>Codec:</strong> ${status.videoInfo.codec || 'Unknown'}</div>
            `;
            
            if (status.videoInfo.fps) {
                html += `<div><strong>FPS:</strong> ${status.videoInfo.fps}</div>`;
            }
            
            if (status.videoInfo.bitrate) {
                html += `<div><strong>Bitrate:</strong> ${Math.round(status.videoInfo.bitrate / 1000)} kbps</div>`;
            }
        }
        
        if (status.hlsPlayback) {
            html += `<div><strong>HLS URL:</strong> <a href="${status.hlsPlayback}" target="_blank">${status.hlsPlayback}</a></div>`;
        }
        
        if (status.rtmpIngest) {
            html += `<div><strong>RTMP Ingest:</strong> ${status.rtmpIngest}</div>`;
        }
        
        streamStatus.innerHTML = html;
    }
    
    // Event listeners
    playButton.addEventListener('click', function() {
        playStream(streamPath.value);
    });
    
    streamPath.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            playStream(streamPath.value);
        }
    });
    
    // Initial load
    fetchStreams();
    
    // Refresh streams every 5 seconds
    setInterval(fetchStreams, 5000);
});