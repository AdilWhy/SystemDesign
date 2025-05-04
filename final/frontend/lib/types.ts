/**
 * Common types used across the application
 */

// Basic stream info for listings
export interface Stream {
  streamId: string;
  userId: string;
  title: string;
  description: string;
  thumbnailUrl: string | null;
  viewerCount: number;
  startedAt: string; // ISO date string
  tags: string[];
}

// Detailed stream info for individual stream pages
export interface StreamDetails extends Stream {
  playbackUrl: string;
  category: string;
  status: StreamStatus;
}

// Stream status enum
export enum StreamStatus {
  CREATED = "CREATED",
  LIVE = "LIVE",
  ENDED = "ENDED",
  ERROR = "ERROR"
}

// Type for API configuration
export interface ApiConfig {
  apiBaseUrl: string;
  wsBaseUrl: string;
}

// User profile information
export interface User {
  id: string;
  username: string;
  displayName: string;
  profileImage: string | null;
  bio: string | null;
  isStreaming: boolean;
}

// Follow relationship
export interface Follow {
  id: string;
  followerId: string;
  followeeId: string;
  createdAt: string; // ISO date string
  notificationsEnabled: boolean;
}

// Chat message
export interface ChatMessage {
  streamId: string;
  userId: string;
  username: string;
  message: string;
  timestamp: number;
  type: ChatMessageType;
}

// Chat message types
export enum ChatMessageType {
  USER_MESSAGE = "USER_MESSAGE",
  SYSTEM_MESSAGE = "SYSTEM_MESSAGE",
  MODERATOR_MESSAGE = "MODERATOR_MESSAGE"
}