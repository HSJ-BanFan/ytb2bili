import { create } from 'zustand';
import type { Video } from '@/types';

interface VideoState {
  videos: Video[];
  currentVideo: Video | null;
  setVideos: (videos: Video[]) => void;
  addVideo: (video: Video) => void;
  updateVideo: (id: number, updates: Partial<Video>) => void;
  setCurrentVideo: (video: Video | null) => void;
  removeVideo: (id: number) => void;
}


// 视频状态管理
export const useVideoStore = create<VideoState>((set) => ({
  videos: [],
  currentVideo: null,
  setVideos: (videos) => set({ videos }),
  addVideo: (video) => set((state) => ({ 
    videos: [video, ...state.videos] 
  })),
  updateVideo: (id, updates) => set((state) => ({
    videos: state.videos.map(video => 
      video.id === id ? { ...video, ...updates } : video
    ),
    currentVideo: state.currentVideo?.id === id 
      ? { ...state.currentVideo, ...updates } 
      : state.currentVideo
  })),
  setCurrentVideo: (video) => set({ currentVideo: video }),
  removeVideo: (id) => set((state) => ({
    videos: state.videos.filter(video => video.id !== id),
    currentVideo: state.currentVideo?.id === id ? null : state.currentVideo
  })),
}));