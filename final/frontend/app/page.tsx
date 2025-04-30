"use client";

import { useState, useEffect } from 'react';
import GrpcStreamCanvas from "@/components/GRPCCanvas";
import { videoApi } from "@/lib/api";

interface Video {
  id: string;
  title: string;
  description: string;
  thumbnail_url?: string;
  video_url?: string;
  view_count: number;
  duration_seconds: number;
}

export default function Home() {
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchVideos = async () => {
      try {
        setLoading(true);
        const data = await videoApi.listVideos();
        setVideos(data.videos || []);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch videos:', err);
        setError('Failed to load videos');
        // Populate with mock data if the API fails
        setVideos([
          {
            id: 'video-1',
            title: 'Sample Gaming Stream',
            description: 'This is a sample gaming stream',
            thumbnail_url: 'https://placehold.co/320x180?text=Gaming',
            view_count: 1200,
            duration_seconds: 3600,
          },
          {
            id: 'video-2',
            title: 'Tech Talk',
            description: 'Discussing the latest in tech',
            thumbnail_url: 'https://placehold.co/320x180?text=Tech',
            view_count: 850,
            duration_seconds: 1800,
          },
          {
            id: 'video-3',
            title: 'Music Session',
            description: 'Live music performance',
            thumbnail_url: 'https://placehold.co/320x180?text=Music',
            view_count: 3500,
            duration_seconds: 2700,
          },
        ]);
      } finally {
        setLoading(false);
      }
    };

    fetchVideos();
  }, []);

  // Format duration from seconds to MM:SS or HH:MM:SS
  const formatDuration = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const remainingSeconds = seconds % 60;

    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
  };

  // Format view count (e.g., 1.2k, 3.4M)
  const formatViewCount = (count: number): string => {
    if (count >= 1000000) {
      return `${(count / 1000000).toFixed(1)}M views`;
    }
    if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}k views`;
    }
    return `${count} views`;
  };

  return (
    <div className="min-h-screen p-8 bg-gray-100">
      <h1 className="text-3xl font-bold mb-8 text-center">Twitch-like Video Streaming Platform</h1>

      {/* Featured Live Stream */}
      <div className="mb-12">
        <h2 className="text-2xl font-bold mb-4">Featured Live Stream</h2>
        <div className="max-w-6xl mx-auto h-[600px] shadow-xl rounded-lg overflow-hidden border border-gray-300">
          <GrpcStreamCanvas />
        </div>
      </div>

      {/* Videos Section */}
      <div className="mb-8">
        <h2 className="text-2xl font-bold mb-4">Recommended Videos</h2>
        
        {error && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
            {error}
          </div>
        )}

        {loading ? (
          <div className="flex justify-center items-center h-40">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-700"></div>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
            {videos.map(video => (
              <div 
                key={video.id} 
                className="bg-white rounded-lg overflow-hidden shadow-md hover:shadow-lg transition-shadow"
              >
                <div className="relative">
                  <img 
                    src={video.thumbnail_url || `https://placehold.co/320x180?text=${encodeURIComponent(video.title)}`} 
                    alt={video.title}
                    className="w-full h-48 object-cover"
                  />
                  <div className="absolute bottom-2 right-2 bg-black bg-opacity-70 text-white text-xs px-2 py-1 rounded">
                    {formatDuration(video.duration_seconds)}
                  </div>
                </div>
                <div className="p-4">
                  <h3 className="font-semibold text-lg mb-1 line-clamp-2">{video.title}</h3>
                  <p className="text-gray-600 text-sm mb-2">{formatViewCount(video.view_count)}</p>
                  <p className="text-gray-500 text-sm line-clamp-2">{video.description}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}