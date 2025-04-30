import { useState, useEffect, useRef } from 'react';
import { streamApi, videoApi } from '@/lib/api';

interface Stream {
  stream_id: string;
  user_id: string;
  title: string;
  description: string;
  thumbnail_url?: string;
  playback_url: string;
  viewer_count: number;
  started_at: string;
  tags?: string[];
}

const GrpcStreamCanvas = () => {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const [isConnected, setIsConnected] = useState(false);
    const [messages, setMessages] = useState<string[]>([]);
    const [viewerCount, setViewerCount] = useState(0);
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [activeStreams, setActiveStreams] = useState<Stream[]>([]);
    const [selectedStream, setSelectedStream] = useState<Stream | null>(null);

    useEffect(() => {
        // Fetch active streams from the backend
        const fetchStreams = async () => {
            try {
                setLoading(true);
                const data = await streamApi.listStreams();
                setActiveStreams(data.streams || []);
                
                // If there are active streams, select the first one
                if (data.streams && data.streams.length > 0) {
                    setSelectedStream(data.streams[0]);
                    setViewerCount(data.streams[0].viewer_count || 0);
                    setIsConnected(true);
                    setMessages(prev => [...prev, `Connected to stream: ${data.streams[0].title}`]);
                }
                setError(null);
            } catch (err) {
                console.error('Failed to fetch streams:', err);
                setError('Failed to load streams. Using mock data instead.');
                
                // Fall back to mock data
                mockStreamConnection();
            } finally {
                setLoading(false);
            }
        };

        fetchStreams();

        // Return cleanup function
        return () => {
            // Any cleanup code here
        };
    }, []);

    const mockStreamConnection = () => {
        setIsConnected(true);
        setViewerCount(Math.floor(Math.random() * 500) + 100);
        
        // Add a mock message
        setMessages(prev => [...prev, `[Mock] Connected to demo stream at ${new Date().toLocaleTimeString()}`]);
    };

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        const resizeCanvas = () => {
            canvas.width = canvas.clientWidth;
            canvas.height = canvas.clientHeight;
        };

        resizeCanvas();
        window.addEventListener('resize', resizeCanvas);

        // Only start the animation if we're connected to a stream
        if (isConnected) {
            const viewerInterval = setInterval(() => {
                setViewerCount(prev => {
                    const change = Math.floor(Math.random() * 10) - 3;
                    return Math.max(50, prev + change);
                });
            }, 3000);

            const dataInterval = setInterval(() => {
                const newData = `[${new Date().toLocaleTimeString()}] ${
                    selectedStream 
                    ? `Stream data from ${selectedStream.title}`
                    : `Data point ${Math.floor(Math.random() * 1000)}`
                }`;
                setMessages(prev => [...prev.slice(-20), newData]);

                drawRandomShape(ctx);
            }, 500);
            
            return () => {
                clearInterval(dataInterval);
                clearInterval(viewerInterval);
            };
        }

        return () => {
            window.removeEventListener('resize', resizeCanvas);
        };
    }, [isConnected, selectedStream]);

    const drawRandomShape = (context: CanvasRenderingContext2D) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        
        context.fillStyle = 'rgba(0, 0, 0, 0.1)';
        context.fillRect(0, 0, canvas.width, canvas.height);

        const x = Math.random() * canvas.width;
        const y = Math.random() * canvas.height;
        const size = 5 + Math.random() * 20;

        context.beginPath();
        context.fillStyle = `hsl(${Math.random() * 360}, 100%, 50%)`;

        if (Math.random() > 0.5) {
            context.arc(x, y, size, 0, Math.PI * 2);
        } else {
            context.rect(x - size / 2, y - size / 2, size, size);
        }

        context.fill();
    };

    // Handle stream selection
    const handleStreamSelect = (stream: Stream) => {
        setSelectedStream(stream);
        setViewerCount(stream.viewer_count || 0);
        setMessages(prev => [...prev, `Switched to stream: ${stream.title}`]);
    };

    // Handle starting a new stream (mock implementation)
    const handleStartStream = async () => {
        try {
            setLoading(true);
            // In a real implementation, we would use the actual user ID
            const mockUserId = "user-" + Math.floor(Math.random() * 1000);
            
            // Get stream key
            const keyResponse = await streamApi.getStreamKey(mockUserId);
            
            // Start the stream
            const streamResponse = await streamApi.startStream({
                userId: mockUserId,
                streamKey: keyResponse.stream_key,
                title: "New Test Stream",
                description: "This is a test stream started from the UI"
            });
            
            // Update UI with new stream
            setSelectedStream({
                stream_id: streamResponse.stream_id,
                user_id: mockUserId,
                title: "New Test Stream",
                description: "This is a test stream started from the UI",
                playback_url: streamResponse.playback_url,
                viewer_count: 0,
                started_at: new Date().toISOString()
            });
            
            setIsConnected(true);
            setMessages(prev => [...prev, `Started new stream with ID: ${streamResponse.stream_id}`]);
            
        } catch (err) {
            console.error('Failed to start stream:', err);
            setError('Failed to start new stream. Using mock connection instead.');
            mockStreamConnection();
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="flex flex-col h-full border border-purple-600 rounded-lg overflow-hidden">
            <div className="bg-purple-900 text-white p-4 flex justify-between items-center">
                <div>
                    <h2 className="text-xl font-bold">
                        {selectedStream ? selectedStream.title : "Video Stream"}
                    </h2>
                    <div className="flex items-center mt-2">
                        <div className={`w-3 h-3 rounded-full mr-2 ${isConnected ? 'bg-red-500 animate-pulse' : 'bg-gray-500'}`}></div>
                        <span className="mr-4">{isConnected ? 'LIVE' : 'OFFLINE'}</span>
                        <div className="flex items-center">
                            <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                <path d="M10 12a2 2 0 100-4 2 2 0 000 4z" />
                                <path fillRule="evenodd" d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clipRule="evenodd" />
                            </svg>
                            <span>{viewerCount.toLocaleString()}</span>
                        </div>
                    </div>
                </div>
                <div className="flex gap-2">
                    {!isConnected && (
                        <button 
                            onClick={handleStartStream}
                            disabled={loading}
                            className="bg-purple-700 hover:bg-purple-600 rounded-lg px-3 py-1 text-sm disabled:opacity-50"
                        >
                            {loading ? 'Starting...' : 'Start Stream'}
                        </button>
                    )}
                    <div className="bg-purple-700 rounded-lg px-3 py-1 text-sm">
                        gRPC Stream
                    </div>
                </div>
            </div>
            
            {error && (
                <div className="bg-red-800 text-white p-2 text-sm">
                    Error: {error}
                </div>
            )}
            
            <div className="flex-1 relative">
                {loading && (
                    <div className="absolute inset-0 flex items-center justify-center bg-black bg-opacity-70 z-10">
                        <div className="text-white">Loading stream data...</div>
                    </div>
                )}
                <canvas
                    ref={canvasRef}
                    className="w-full h-full bg-black"
                />
            </div>
            
            {activeStreams.length > 0 && (
                <div className="bg-gray-800 p-2 flex gap-2 overflow-x-auto">
                    {activeStreams.map(stream => (
                        <button
                            key={stream.stream_id}
                            onClick={() => handleStreamSelect(stream)}
                            className={`px-3 py-1 rounded text-xs whitespace-nowrap ${
                                selectedStream?.stream_id === stream.stream_id
                                    ? 'bg-purple-600 text-white'
                                    : 'bg-gray-700 text-gray-200 hover:bg-gray-600'
                            }`}
                        >
                            {stream.title} ({stream.viewer_count} viewers)
                        </button>
                    ))}
                </div>
            )}
            
            <div className="bg-gray-900 text-green-400 p-2 h-48 overflow-y-auto font-mono text-sm">
                <div className="flex justify-between items-center mb-2 text-white">
                    <span>Chat</span>
                    <div className="flex space-x-2">
                        <button className="bg-purple-700 hover:bg-purple-600 px-2 py-1 rounded text-xs">❤️</button>
                        <button className="bg-purple-700 hover:bg-purple-600 px-2 py-1 rounded text-xs">👍</button>
                        <button className="bg-purple-700 hover:bg-purple-600 px-2 py-1 rounded text-xs">🎉</button>
                    </div>
                </div>
                {messages.map((msg, i) => (
                    <div key={i} className="leading-tight">
                        <span className="text-purple-400 font-bold">System:</span> {msg}
                    </div>
                ))}
            </div>
        </div >
    );
};

export default GrpcStreamCanvas;