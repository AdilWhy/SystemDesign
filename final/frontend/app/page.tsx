"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";

// Types for our API responses
interface Stream {
  streamId: string;
  userId: string;
  title: string;
  description: string;
  thumbnailUrl: string;
  viewerCount: number;
  startedAt: string;
}

// API response type to match your Go backend
interface ApiStream {
  stream_id: string;
  user_id: string;
  title: string;
  description: string;
  thumbnail_url: string;
  viewer_count: number;
  started_at: string;
  playback_url: string;
  tags: string[];
}

export default function Home() {
  const [activeStreams, setActiveStreams] = useState<Stream[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fetch active streams on component mount
    const fetchStreams = async () => {
      try {
        setIsLoading(true);
        // In a real implementation, this would be a call to your API
        const response = await fetch("http://localhost:8080/api/v1/streams");
        
        if (!response.ok) {
          throw new Error("Failed to fetch streams");
        }
        
        const data = await response.json();
        
        // Map API response (snake_case) to our frontend model (camelCase)
        const mappedStreams = (data.streams || []).map((apiStream: ApiStream) => ({
          streamId: apiStream.stream_id,
          userId: apiStream.user_id,
          title: apiStream.title,
          description: apiStream.description,
          thumbnailUrl: apiStream.thumbnail_url,
          viewerCount: apiStream.viewer_count,
          startedAt: apiStream.started_at
        }));
        
        setActiveStreams(mappedStreams);
      } catch (err) {
        console.error("Error fetching streams:", err);
        setError(err instanceof Error ? err.message : "An unknown error occurred");
      } finally {
        setIsLoading(false);
      }
    };

    fetchStreams();
    
    // Poll for new streams every 30 seconds
    const interval = setInterval(fetchStreams, 30000);
    
    // Clean up interval on component unmount
    return () => clearInterval(interval);
  }, []);

  return (
    <main className="flex min-h-screen flex-col items-center p-6 bg-gray-900 text-white">
      <div className="w-full max-w-6xl">
        <h1 className="text-4xl font-bold mb-8">Live Streams</h1>
        
        {isLoading && <p className="text-center text-lg">Loading streams...</p>}
        
        {error && (
          <div className="bg-red-500 text-white p-4 rounded-md mb-6">
            {error}
          </div>
        )}
        
        {!isLoading && activeStreams.length === 0 && (
          <div className="text-center py-12">
            <h2 className="text-xl mb-4">No active streams right now</h2>
            <p className="mb-6">Be the first to start streaming!</p>
            <Link 
              href="/stream/create" 
              className="bg-purple-600 hover:bg-purple-700 px-6 py-3 rounded-md font-medium transition"
            >
              Start Streaming
            </Link>
          </div>
        )}
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {activeStreams.map((stream) => (
            <Link 
              href={`/stream/${stream.streamId}`} 
              key={stream.streamId}
              className="bg-gray-800 rounded-lg overflow-hidden hover:ring-2 hover:ring-purple-500 transition"
            >
              <div className="relative h-48 w-full">
                {stream.thumbnailUrl ? (
                  <Image
                    src={stream.thumbnailUrl}
                    alt={stream.title}
                    fill
                    className="object-cover"
                  />
                ) : (
                  <div className="w-full h-full bg-gray-700 flex items-center justify-center">
                    <span className="text-gray-500">No thumbnail</span>
                  </div>
                )}
                <div className="absolute top-2 right-2 bg-red-600 px-2 py-1 rounded text-sm font-medium">
                  LIVE
                </div>
                <div className="absolute bottom-2 right-2 bg-black bg-opacity-70 px-2 py-1 rounded text-sm">
                  {stream.viewerCount} viewers
                </div>
              </div>
              <div className="p-4">
                <h3 className="font-bold text-lg mb-1 truncate">{stream.title}</h3>
                <p className="text-gray-400 text-sm mb-2 truncate">{stream.description}</p>
                <p className="text-gray-500 text-xs">
                  Started {new Date(stream.startedAt).toLocaleTimeString()}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </main>
  );
}