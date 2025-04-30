import { config } from './utils';

// API service to communicate with the backend
const API_BASE_URL = config.apiBaseUrl;

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