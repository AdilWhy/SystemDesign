"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { getStreamKey, startStream } from "@/lib/api";

export default function CreateStreamPage() {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [tags, setTags] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [streamKey, setStreamKey] = useState<string | null>(null);
  const [rtmpUrl, setRtmpUrl] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      // For demo purposes, we'll use a dummy user ID
      const userId = "user123";
      
      // Step 1: Get a stream key from the server
      const keyData = await getStreamKey(userId);
      const streamKey = keyData.stream_key;
      const rtmpUrl = keyData.rtmp_url;
      
      // Step 2: Create the stream
      const streamData = await startStream({
        userId,
        streamKey,
        title,
        description,
        tags: tags.split(",").map(tag => tag.trim()).filter(tag => tag),
      });
      
      // Display stream key and RTMP URL to the user
      setStreamKey(streamKey);
      setRtmpUrl(rtmpUrl);

    } catch (err) {
      console.error("Error creating stream:", err);
      setError(err instanceof Error ? err.message : "An unknown error occurred");
    } finally {
      setIsLoading(false);
    }
  };

  if (streamKey && rtmpUrl) {
    return (
      <div className="min-h-screen bg-gray-900 text-white p-6">
        <div className="max-w-2xl mx-auto bg-gray-800 p-6 rounded-lg">
          <h1 className="text-2xl font-bold mb-6">Stream Created!</h1>
          
          <div className="mb-6">
            <p className="mb-4">Use the following information in your streaming software (like OBS):</p>
            
            <div className="mb-4">
              <h2 className="font-medium mb-2">RTMP URL:</h2>
              <div className="bg-gray-700 p-4 rounded-md flex justify-between items-center">
                <code className="text-green-400">{rtmpUrl}</code>
                <button 
                  onClick={() => navigator.clipboard.writeText(rtmpUrl)}
                  className="bg-gray-600 hover:bg-gray-500 px-3 py-1 rounded-md text-sm"
                >
                  Copy
                </button>
              </div>
            </div>
            
            <div>
              <h2 className="font-medium mb-2">Stream Key:</h2>
              <div className="bg-gray-700 p-4 rounded-md flex justify-between items-center">
                <code className="text-green-400">{streamKey}</code>
                <button 
                  onClick={() => navigator.clipboard.writeText(streamKey)}
                  className="bg-gray-600 hover:bg-gray-500 px-3 py-1 rounded-md text-sm"
                >
                  Copy
                </button>
              </div>
              <p className="mt-2 text-yellow-400 text-sm">Keep your stream key secret! Anyone with this key can stream to your channel.</p>
            </div>
          </div>
          
          <div className="flex gap-4">
            <Link 
              href="/"
              className="bg-gray-700 hover:bg-gray-600 px-4 py-2 rounded-md"
            >
              Return to Home
            </Link>
            <Link 
              href="/dashboard"
              className="bg-purple-600 hover:bg-purple-700 px-4 py-2 rounded-md"
            >
              Go to Dashboard
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white p-6">
      <div className="max-w-2xl mx-auto">
        <h1 className="text-3xl font-bold mb-8">Create a New Stream</h1>
        
        {error && (
          <div className="bg-red-500 text-white p-4 rounded-md mb-6">
            {error}
          </div>
        )}
        
        <form onSubmit={handleSubmit} className="bg-gray-800 p-6 rounded-lg">
          <div className="mb-4">
            <label htmlFor="title" className="block mb-2 font-medium">
              Stream Title*
            </label>
            <input
              id="title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full bg-gray-700 rounded-md px-4 py-2 text-white outline-none focus:ring-2 focus:ring-purple-500"
              required
              maxLength={100}
              placeholder="Enter a catchy title for your stream"
            />
          </div>
          
          <div className="mb-4">
            <label htmlFor="description" className="block mb-2 font-medium">
              Description
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full bg-gray-700 rounded-md px-4 py-2 text-white outline-none focus:ring-2 focus:ring-purple-500 h-32 resize-none"
              placeholder="Tell viewers what your stream is about"
            />
          </div>
          
          <div className="mb-4">
            <label htmlFor="category" className="block mb-2 font-medium">
              Category
            </label>
            <select
              id="category"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              className="w-full bg-gray-700 rounded-md px-4 py-2 text-white outline-none focus:ring-2 focus:ring-purple-500"
            >
              <option value="">Select a category</option>
              <option value="gaming">Gaming</option>
              <option value="irl">IRL</option>
              <option value="music">Music</option>
              <option value="creative">Creative</option>
              <option value="esports">Esports</option>
              <option value="talk_shows">Talk Shows</option>
            </select>
          </div>
          
          <div className="mb-6">
            <label htmlFor="tags" className="block mb-2 font-medium">
              Tags
            </label>
            <input
              id="tags"
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              className="w-full bg-gray-700 rounded-md px-4 py-2 text-white outline-none focus:ring-2 focus:ring-purple-500"
              placeholder="Enter tags separated by commas (e.g., fps, competitive, speedrun)"
            />
            <p className="mt-1 text-sm text-gray-400">
              Tags help viewers find your stream
            </p>
          </div>
          
          <div className="flex justify-end">
            <Link
              href="/"
              className="bg-gray-700 hover:bg-gray-600 px-6 py-2 rounded-md mr-4"
            >
              Cancel
            </Link>
            <button
              type="submit"
              className="bg-purple-600 hover:bg-purple-700 px-6 py-2 rounded-md"
              disabled={isLoading || !title.trim()}
            >
              {isLoading ? (
                <span className="flex items-center">
                  <span className="animate-spin h-4 w-4 mr-2 border-t-2 border-b-2 border-white rounded-full"></span>
                  Creating...
                </span>
              ) : (
                "Create Stream"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}