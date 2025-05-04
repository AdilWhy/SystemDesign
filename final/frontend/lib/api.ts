import { config } from './utils';
import { Stream, StreamDetails } from "./types";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || config.apiBaseUrl || "http://localhost:8080/api/v1";

// Helper function to handle API requests with better debugging
const apiRequest = async (url: string, options?: RequestInit) => {
  console.log(`API Request: ${options?.method || 'GET'} ${url}`, options?.body ? JSON.parse(options.body as string) : '');
  
  try {
    const response = await fetch(url, {
      ...options,
      // Adding these headers often helps with CORS issues
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        ...(options?.headers || {})
      },
    });
    
    // Log response status
    console.log(`API Response: ${response.status} ${response.statusText} from ${url}`);
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API Error: ${response.status} ${response.statusText}`, errorText);
      throw new Error(`${response.status} ${response.statusText}: ${errorText}`);
    }
    
    // For empty responses or 204 No Content
    if (response.status === 204 || response.headers.get('content-length') === '0') {
      return null;
    }
    
    const data = await response.json();
    console.log('API Response Data:', data);
    return data;
  } catch (error) {
    console.error('API Request Failed:', error);
    throw error;
  }
};

// API client for the video streaming platform
export const api = {
  // Fetches active streams
  async getActiveStreams(): Promise<Stream[]> {
    try {
      const response = await fetch(`${API_BASE_URL}/streams`);
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      const data = await response.json();
      return data.streams || [];
    } catch (error) {
      console.error("Failed to fetch streams:", error);
      throw error;
    }
  },
  
  // Fetches details for a specific stream
  async getStreamDetails(streamId: string): Promise<StreamDetails> {
    try {
      const response = await fetch(`${API_BASE_URL}/streams/${streamId}`);
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      return await response.json();
    } catch (error) {
      console.error(`Failed to fetch stream ${streamId}:`, error);
      throw error;
    }
  },
  
  // Creates a new stream
  async createStream(streamData: {
    userId: string;
    title: string;
    description?: string;
    category?: string;
    tags?: string[];
    streamKey: string;
  }): Promise<{ streamId: string; playbackUrl: string }> {
    try {
      const response = await fetch(`${API_BASE_URL}/streams`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(streamData),
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      return await response.json();
    } catch (error) {
      console.error("Failed to create stream:", error);
      throw error;
    }
  },
  
  // Gets a stream key for a user
  async getStreamKey(userId: string): Promise<{ streamKey: string; rtmpUrl: string }> {
    try {
      const response = await fetch(`${API_BASE_URL}/streams/key`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ user_id: userId }),
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      return await response.json();
    } catch (error) {
      console.error("Failed to get stream key:", error);
      throw error;
    }
  },
  
  // Ends a stream
  async endStream(streamId: string, userId: string): Promise<void> {
    try {
      const response = await fetch(`${API_BASE_URL}/streams/${streamId}`, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ userId }),
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
    } catch (error) {
      console.error(`Failed to end stream ${streamId}:`, error);
      throw error;
    }
  },
  
  // Follows a channel
  async followChannel(followerId: string, followeeId: string): Promise<void> {
    try {
      const response = await fetch(`${API_BASE_URL}/follows`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          followerId,
          followeeId,
        }),
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
    } catch (error) {
      console.error(`Failed to follow channel ${followeeId}:`, error);
      throw error;
    }
  },
  
  // Unfollows a channel
  async unfollowChannel(followId: string): Promise<void> {
    try {
      const response = await fetch(`${API_BASE_URL}/follows/${followId}`, {
        method: "DELETE",
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
    } catch (error) {
      console.error(`Failed to unfollow channel ${followId}:`, error);
      throw error;
    }
  },
  
  // Gets followers for a user
  async getUserFollowers(userId: string): Promise<string[]> {
    try {
      const response = await fetch(`${API_BASE_URL}/users/${userId}/followers`);
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      const data = await response.json();
      return data.followers || [];
    } catch (error) {
      console.error(`Failed to get followers for ${userId}:`, error);
      throw error;
    }
  },
  
  // Gets channels a user follows
  async getUserFollowing(userId: string): Promise<string[]> {
    try {
      const response = await fetch(`${API_BASE_URL}/users/${userId}/following`);
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      const data = await response.json();
      return data.following || [];
    } catch (error) {
      console.error(`Failed to get following for ${userId}:`, error);
      throw error;
    }
  },
  
  // Checks if a user follows a channel
  async checkFollowStatus(followerId: string, followeeId: string): Promise<boolean> {
    try {
      const following = await this.getUserFollowing(followerId);
      return following.includes(followeeId);
    } catch (error) {
      console.error(`Failed to check follow status:`, error);
      return false;
    }
  }
};

// Video related API calls
export const videoApi = {
  // Get a list of videos
  listVideos: async () => {
    return apiRequest(`${API_BASE_URL}/videos`);
  },

  // Get a single video by ID
  getVideo: async (videoId: string) => {
    return apiRequest(`${API_BASE_URL}/videos/${videoId}`);
  },

  // Initiate a new video upload
  initiateUpload: async (data: {
    title: string;
    description: string;
    userId: string;
    fileSizeBytes: number;
    contentType: string;
  }) => {
    return apiRequest(`${API_BASE_URL}/videos`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  // Complete a video upload
  completeUpload: async (videoId: string, uploadId: string) => {
    return apiRequest(`${API_BASE_URL}/videos/${videoId}/complete`, {
      method: 'POST',
      body: JSON.stringify({ upload_id: uploadId }),
    });
  },

  // Delete a video
  deleteVideo: async (videoId: string, userId: string) => {
    return apiRequest(`${API_BASE_URL}/videos/${videoId}`, {
      method: 'DELETE',
      body: JSON.stringify({ user_id: userId }),
    });
  },
};

// Streaming related API calls
export const streamApi = {
  // Get stream key for a user
  getStreamKey: async (userId: string) => {
    return apiRequest(`${API_BASE_URL}/streams/key`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId }),
    });
  },

  // Start a new stream
  startStream: async (data: {
    userId: string;
    streamKey: string;
    title: string;
    description: string;
    tags?: string[];
  }) => {
    return apiRequest(`${API_BASE_URL}/streams`, {
      method: 'POST',
      body: JSON.stringify({
        user_id: data.userId,
        stream_key: data.streamKey,
        title: data.title,
        description: data.description,
        tags: data.tags || [],
      }),
    });
  },

  // End a stream
  endStream: async (streamId: string, userId: string) => {
    return apiRequest(`${API_BASE_URL}/streams/${streamId}`, {
      method: 'DELETE',
      body: JSON.stringify({ user_id: userId }),
    });
  },

  // List active streams
  listStreams: async () => {
    return apiRequest(`${API_BASE_URL}/streams`);
  },
};

/**
 * Fetch a stream key for the current user
 */
export async function getStreamKey(userId: string): Promise<{ stream_key: string; rtmp_url: string }> {
  const response = await fetch(`${API_BASE_URL}/streams/key`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ user_id: userId }),
  });

  if (!response.ok) {
    throw new Error('Failed to get stream key');
  }

  return response.json();
}

/**
 * Start a new live stream
 */
export async function startStream(params: {
  userId: string;
  streamKey: string;
  title: string;
  description: string;
  tags: string[];
}): Promise<{ stream_id: string; playback_url: string; stream_key: string }> {
  const response = await fetch(`${API_BASE_URL}/streams`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: params.userId,
      stream_key: params.streamKey,
      title: params.title,
      description: params.description,
      tags: params.tags,
    }),
  });

  if (!response.ok) {
    throw new Error('Failed to start stream');
  }

  return response.json();
}

/**
 * End a live stream
 */
export async function endStream(streamId: string, userId: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/streams/${streamId}`, {
    method: 'DELETE',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ user_id: userId }),
  });

  if (!response.ok) {
    throw new Error('Failed to end stream');
  }
}

/**
 * Get live stream details
 */
export async function getLiveStream(streamId: string): Promise<any> {
  console.log(`Fetching stream details for ID: ${streamId}`);
  
  if (!streamId || streamId === "undefined") {
    throw new Error("Invalid stream ID provided");
  }
  
  try {
    // Use the more robust apiRequest helper instead of fetch directly
    return await apiRequest(`${API_BASE_URL}/streams/${streamId}`);
  } catch (error) {
    console.error(`Failed to get stream details for ID ${streamId}:`, error);
    throw error; // Re-throw to be handled by the component
  }
}

/**
 * List active live streams
 */
export async function getLiveStreams(params?: { userId?: string; pageSize?: number; pageToken?: string }): Promise<any> {
  const queryParams = new URLSearchParams();
  if (params?.userId) queryParams.set('user_id', params.userId);
  if (params?.pageSize) queryParams.set('page_size', params.pageSize.toString());
  if (params?.pageToken) queryParams.set('page_token', params.pageToken);

  const response = await fetch(`${API_BASE_URL}/streams?${queryParams.toString()}`);

  if (!response.ok) {
    throw new Error('Failed to list streams');
  }

  return response.json();
}