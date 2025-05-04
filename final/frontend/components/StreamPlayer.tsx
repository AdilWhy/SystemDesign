"use client";

import { useEffect, useRef, useState } from "react";
import Hls from "hls.js";

interface StreamPlayerProps {
  streamId: string;
  streamUrl: string;
}

export default function StreamPlayer({ streamId, streamUrl }: StreamPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const video = videoRef.current;
    
    if (!video) return;
    
    let hls: Hls | null = null;

    const setupHlsPlayer = async () => {
      setIsLoading(true);
      setError(null);

      try {
        console.log(`Setting up HLS player with stream URL: ${streamUrl}`);
        
        // Check if the URL is valid
        if (!streamUrl || streamUrl === "undefined") {
          setError("Invalid stream URL");
          setIsLoading(false);
          return;
        }

        // Check if HLS is supported natively
        if (video.canPlayType("application/vnd.apple.mpegurl")) {
          // Use native HLS support
          video.src = streamUrl;
          video.addEventListener('loadedmetadata', () => {
            console.log("Video metadata loaded (native HLS)");
            setIsLoading(false);
            video.play().catch(playError => {
              console.warn("Auto-play failed:", playError);
            });
          });
          
          video.addEventListener('error', () => {
            const errorCode = video.error?.code || 0;
            const errorMessage = video.error?.message || 'Unknown error';
            console.error(`Video error (native HLS): ${errorCode} - ${errorMessage}`);
            setError(`Playback error: ${errorMessage}`);
            setIsLoading(false);
          });
        } else if (Hls.isSupported()) {
          // Use HLS.js with improved configuration
          hls = new Hls({
            enableWorker: true,
            lowLatencyMode: true,
            backBufferLength: 60,
            maxBufferLength: 30,
            maxMaxBufferLength: 60,
            maxBufferSize: 10 * 1000 * 1000, // 10MB
            maxBufferHole: 0.5,
            liveSyncDurationCount: 3,
            liveMaxLatencyDurationCount: 10,
            debug: false,
            // Advanced recovery options
            fragLoadingRetryDelay: 1000,
            manifestLoadingRetryDelay: 1000,
            levelLoadingRetryDelay: 1000,
            fragLoadingMaxRetry: 6,
            manifestLoadingMaxRetry: 6,
            levelLoadingMaxRetry: 6,
          });
          
          hls.loadSource(streamUrl);
          hls.attachMedia(video);
          
          hls.on(Hls.Events.MANIFEST_PARSED, () => {
            console.log("HLS manifest parsed successfully");
            setIsLoading(false);
            video.play().catch(err => {
              console.warn("Auto-play failed:", err);
            });
          });
          
          hls.on(Hls.Events.ERROR, (_, data) => {
            // Ultra-defensive error handling to prevent console errors
            try {
              // Avoid any direct property access and use optional chaining
              // Log minimal information to avoid console errors
              console.warn("HLS player error detected");
              
              // Create safe copies of data with defensive checks
              let safeErrorInfo = {
                type: typeof data?.type === 'string' ? data.type : 'Unknown',
                details: typeof data?.details === 'string' ? data.details : 'Unknown',
                fatal: Boolean(data?.fatal),
                status: 'unknown'
              };
              
              // Extremely careful with nested properties
              if (data && 
                  typeof data === 'object' && 
                  data.response && 
                  typeof data.response === 'object' && 
                  typeof data.response.code === 'number') {
                safeErrorInfo.status = data.response.code.toString();
              }
              
              // Log the safe error info object instead of individual properties
              console.warn("Error details:", safeErrorInfo);
              
              // Handle manifest errors with type checking
              const isManifestError = typeof data?.details === 'string' && 
                  (data.details.indexOf('MANIFEST') >= 0 || 
                   data.details === Hls.ErrorDetails.MANIFEST_LOAD_ERROR ||
                   data.details === Hls.ErrorDetails.MANIFEST_LOAD_TIMEOUT);
              
              if (isManifestError) {
                setError("Stream not available - please try again later");
                setIsLoading(false);
                
                // For manifest errors, try reloading after a delay
                setTimeout(() => {
                  if (hls) {
                    try {
                      console.log("Attempting to reload source after manifest error");
                      hls.loadSource(streamUrl);
                      hls.startLoad();
                    } catch (e) {
                      // Silent catch
                    }
                  }
                }, 5000);
                return; // Exit early
              }
              
              // Handle fatal errors with type checking
              if (data && data.fatal === true) {
                // Safe type check for network errors
                const isNetworkError = data.type === Hls.ErrorTypes.NETWORK_ERROR;
                const isMediaError = data.type === Hls.ErrorTypes.MEDIA_ERROR;
                
                if (isNetworkError) {
                  setError("Network error - attempting to reconnect...");
                  
                  setTimeout(() => {
                    if (hls) {
                      try {
                        hls.startLoad();
                      } catch (e) {
                        // Silent catch
                      }
                    }
                  }, 3000);
                } else if (isMediaError) {
                  setError("Media error - attempting to recover...");
                  
                  setTimeout(() => {
                    if (hls) {
                      try {
                        hls.recoverMediaError();
                      } catch (e) {
                        // Silent catch
                      }
                    }
                  }, 2000);
                } else {
                  setError("Stream playback error - please refresh");
                  
                  // Last resort recovery
                  setTimeout(() => {
                    if (hls) {
                      try {
                        // Completely recreate the HLS instance
                        hls.destroy();
                        
                        // Create new instance with maximum retry settings
                        const newHls = new Hls({
                          enableWorker: true,
                          lowLatencyMode: true,
                          manifestLoadingMaxRetry: 8,
                          manifestLoadingRetryDelay: 2000,
                          fragLoadingMaxRetry: 8
                        });
                        
                        newHls.loadSource(streamUrl);
                        newHls.attachMedia(video);
                        
                        // Update the reference
                        hls = newHls;
                        
                        console.log("Created new HLS instance as last recovery attempt");
                      } catch (e) {
                        // Silent catch
                      }
                    }
                  }, 5000);
                }
              }
            } catch (errorHandlingError) {
              // Catch-all for any errors in our error handler itself
              console.warn("Error in error handler:", errorHandlingError);
              setError("Stream playback error");
              setIsLoading(false);
            }
          });
          
          // Log events for debugging
          hls.on(Hls.Events.MEDIA_ATTACHED, () => {
            console.log("HLS media attached successfully");
          });
          
          hls.on(Hls.Events.MANIFEST_LOADED, (_, data) => {
            console.log("HLS manifest loaded successfully", data);
          });
          
          hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
            console.log(`HLS level loaded: ${data.level}, duration: ${data.details.totalduration}`);
          });
          
          hls.on(Hls.Events.FRAG_LOADED, (_, data) => {
            // Log fragment loading in a non-spammy way (every 5th fragment)
            if (data.frag.sn % 5 === 0) {
              console.log(`HLS fragment loaded: ${data.frag.sn}`);
            }
          });
        } else {
          setError("Your browser does not support HLS playback");
          setIsLoading(false);
        }
      } catch (e) {
        console.error("Error setting up HLS player:", e);
        setError("Failed to load video stream");
        setIsLoading(false);
      }
    };

    setupHlsPlayer();

    // Cleanup
    return () => {
      if (hls) {
        hls.destroy();
      }
      video.src = ""; // Clear the video source
    };
  }, [streamUrl]);

  return (
    <div className="relative w-full bg-black rounded-lg overflow-hidden">
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-black bg-opacity-50 z-10">
          <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-purple-500"></div>
        </div>
      )}
      
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black bg-opacity-50 z-10">
          <div className="bg-red-500 text-white p-4 rounded-md max-w-sm text-center">
            {error}
          </div>
        </div>
      )}
      
      <video 
        ref={videoRef}
        className="w-full h-full"
        controls
        playsInline
      />
      
      <div className="absolute bottom-4 left-4 bg-black bg-opacity-70 px-2 py-1 rounded text-sm text-white">
        Stream ID: {streamId}
      </div>
    </div>
  );
}