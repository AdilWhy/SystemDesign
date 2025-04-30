import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Utils and helpers for the application

// Environment-specific configuration 
export const config = {
  apiBaseUrl: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1',
  grpcWebProxyUrl: process.env.NEXT_PUBLIC_GRPC_WEB_PROXY_URL || 'http://localhost:8080',
};

// Format date to a readable string
export const formatDate = (dateString: string): string => {
  const date = new Date(dateString);
  return date.toLocaleString();
};

// Format number as a readable count (e.g. 1.2k, 3.4M)
export const formatCount = (count: number): string => {
  if (count >= 1000000) {
    return `${(count / 1000000).toFixed(1)}M`;
  }
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}k`;
  }
  return count.toString();
};

// Generate a random color in HSL format
export const randomColor = (): string => {
  return `hsl(${Math.random() * 360}, 70%, 60%)`;
};
