"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import StreamPlayer from "@/components/StreamPlayer";
import Link from "next/link";
import { getLiveStream } from "@/lib/api";

interface StreamDetails {
  streamId: string;
  userId: string;
  title: string;
  description: string;
  playbackUrl: string;
  viewerCount: number;
  startedAt: string;
}

// API response type to match your Go backend
interface ApiStreamDetails {
  stream_id: string;
  user_id: string;
  title: string;
  description: string;
  playback_url: string;
  viewer_count: number;
  started_at: string;
  thumbnail_url?: string;
  tags?: string[];
}

export default function StreamPage() {
  const params = useParams();
  const streamId = params.id as string;
  
  const [stream, setStream] = useState<StreamDetails | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isFollowing, setIsFollowing] = useState(false);

  useEffect(() => {
    let interval: NodeJS.Timeout | null = null;
    
    const fetchStreamDetails = async () => {
      try {
        if (!streamId || streamId === "undefined") {
          throw new Error("Invalid stream ID");
        }
        
        console.log(`Fetching stream details for ID: ${streamId}`);
        const data = await getLiveStream(streamId);
        console.log("Stream data received:", data);
        
        // Make sure the API is returning a valid playback URL
        if (!data.playback_url) {
          console.error("API returned data without playback_url:", data);
          throw new Error("Stream data is missing playback URL");
        }
        
        // Ensure we're using the stream_key as the stream ID for MediaMTX
        // This is the fix for the ID mismatch issue
        const mediaServerStreamId = data.stream_key || data.stream_id;
        
        setStream({
          streamId: data.stream_id,
          userId: data.user_id,
          title: data.title,
          description: data.description,
          // Use the stream_key for the playback URL if available, otherwise fallback to stream_id
          playbackUrl: data.playback_url.replace(data.stream_id, mediaServerStreamId),
          viewerCount: data.viewer_count,
          startedAt: data.started_at,
        });
      } catch (err) {
        console.error("Error fetching stream:", err);
        setError(err instanceof Error ? err.message : "An unknown error occurred");
        setStream(null);
      } finally {
        setIsLoading(false);
      }
    };

    const setupPolling = () => {
      // Initial fetch
      fetchStreamDetails();
      
      // Update viewer count periodically
      interval = setInterval(fetchStreamDetails, 60000);
    };

    if (streamId) {
      setIsLoading(true);
      setError(null);
      setupPolling();
    } else {
      setError("No stream ID provided");
      setIsLoading(false);
    }
    
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [streamId]); // Only depend on streamId, not stream

  const handleFollowToggle = async () => {
    try {
      // In a real implementation, this would call your API
      if (isFollowing) {
        // Unfollow API call
        // await fetch(`/api/follow/${stream?.userId}`, { method: 'DELETE' });
      } else {
        // Follow API call
        // await fetch(`/api/follow/${stream?.userId}`, { method: 'POST' });
      }
      
      // Toggle state optimistically
      setIsFollowing(!isFollowing);
    } catch (err) {
      console.error("Error toggling follow:", err);
      // Reset to previous state on error
      setIsFollowing(isFollowing);
      alert("Failed to update follow status");
    }
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-900 text-white">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-purple-500"></div>
      </div>
    );
  }

  if (error || !stream) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-6 bg-gray-900 text-white">
        <div className="bg-red-500 text-white p-4 rounded-md mb-6 max-w-md text-center">
          {error || "Stream not found"}
        </div>
        <Link href="/" className="text-purple-400 hover:text-purple-300">
          Return to home page
        </Link>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white p-6">
      <div className="max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <StreamPlayer streamId={stream.streamId} streamUrl={stream.playbackUrl} />
          
          <div className="mt-4">
            <h1 className="text-2xl font-bold">{stream.title}</h1>
            <div className="flex justify-between items-center mt-2">
              <p className="text-gray-400">
                {stream.viewerCount} {stream.viewerCount === 1 ? "viewer" : "viewers"}
              </p>
              <p className="text-gray-400">
                Started {new Date(stream.startedAt).toLocaleString()}
              </p>
            </div>
            
            <div className="flex justify-between items-center mt-4 border-t border-gray-800 pt-4">
              <div>
                <h2 className="font-medium">Channel: User-{stream.userId}</h2>
              </div>
              <button
                onClick={handleFollowToggle}
                className={`px-4 py-2 rounded-md ${
                  isFollowing
                    ? "bg-gray-700 hover:bg-gray-600"
                    : "bg-purple-600 hover:bg-purple-700"
                }`}
              >
                {isFollowing ? "Unfollow" : "Follow"}
              </button>
            </div>
            
            <div className="mt-6">
              <h3 className="font-medium mb-2">Description</h3>
              <p className="text-gray-300">{stream.description || "No description provided"}</p>
            </div>
          </div>
        </div>
        
        <div className="bg-gray-800 rounded-lg p-4">
          <h3 className="font-medium mb-4">Chat</h3>
          <div className="h-96 bg-gray-900 rounded-md mb-4 p-4 overflow-y-auto">
            <p className="text-gray-500 text-center">
              Chat is not implemented in this demo
            </p>
          </div>
          <div className="flex">
            <input
              type="text"
              placeholder="Send a message"
              className="flex-1 bg-gray-700 rounded-l-md px-4 py-2 outline-none"
              disabled
            />
            <button className="bg-purple-600 rounded-r-md px-4 py-2 disabled:bg-purple-800" disabled>
              Send
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}