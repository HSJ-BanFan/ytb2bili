import axios from "axios";
import type {
  ApiResponse,
  Video,
  VideoDetail,
  TaskStep,
  VideoFile,
  QRCodeResponse,
  LoginStatus,
  VideoSubmissionRequest,
  UploadValidation,
  MembershipInfo,
  TierConfig,
  QuotaInfo,
  AvailableFeatures,
  FeatureCheckResult,
  BoostPackStatus,
  PurchaseBoostPackRequest,
  PurchaseBoostPackResponse,
} from "@/types";

const API_BASE_URL =
  process.env.NODE_ENV === "development"
    ? "/api/v1" // 开发模式下使用代理
    : process.env.NEXT_PUBLIC_API_URL || "http://localhost:8096/api/v1";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    "Content-Type": "application/json",
  },
});

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    if (typeof window !== "undefined") {
      // 优先使用 JWT Token 进行认证
      const jwtToken = localStorage.getItem("jwt_token");
      if (jwtToken) {
        config.headers["Authorization"] = `Bearer ${jwtToken}`;
        
        // 同时从 JWT 用户信息获取用户 ID
        const jwtUserStr = localStorage.getItem("jwt_user");
        if (jwtUserStr) {
          try {
            const jwtUser = JSON.parse(jwtUserStr);
            if (jwtUser && jwtUser.id) {
              config.headers["X-User-ID"] = jwtUser.id;
            }
          } catch (e) {
            console.error("解析 JWT 用户信息失败:", e);
          }
        }
      }
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    return response.data;
  },
  (error) => {
    console.error("API Error:", error);
    return Promise.reject(error);
  }
);

// JWT 用户信息类型
interface JWTUser {
  id: number;
  username: string;
  email: string;
  membership_tier: string;
}

// JWT Token 响应类型
interface JWTTokenResponse {
  access_token: string;
  refresh_token: string;
  expires_at: string;
  token_type: string;
}

// JWT 登录响应类型
interface JWTLoginResponse {
  token: JWTTokenResponse;
  user: JWTUser;
}

// B站认证相关 API（扫码登录）
export const authApi = {
  // 获取登录二维码
  getQRCode: (): Promise<ApiResponse<QRCodeResponse>> => {
    return api.get("/auth/qrcode");
  },

  // 检查登录状态
  checkLoginStatus: (authCode: string): Promise<ApiResponse<LoginStatus>> => {
    return api.post("/auth/check", { auth_code: authCode });
  },

  // 获取当前用户状态
  getUserStatus: (): Promise<ApiResponse<LoginStatus>> => {
    return api.get("/auth/status");
  },

  // 退出登录
  logout: (): Promise<ApiResponse> => {
    return api.post("/auth/logout");
  },

  // JWT 用户注册
  register: (data: { username: string; email: string; password: string }): Promise<ApiResponse<JWTLoginResponse>> => {
    return api.post("/user/register", data);
  },

  // JWT 用户登录
  login: (data: { username: string; password: string }): Promise<ApiResponse<JWTLoginResponse>> => {
    return api.post("/user/login", data);
  },

  // JWT Token 刷新
  refreshToken: (refreshToken: string): Promise<ApiResponse<JWTLoginResponse>> => {
    return api.post("/user/refresh", { refresh_token: refreshToken });
  },

  // JWT 获取当前用户信息
  getJWTUser: (): Promise<ApiResponse<JWTUser>> => {
    return api.get("/user/me");
  },

  // JWT 退出登录
  jwtLogout: (): Promise<ApiResponse> => {
    return api.post("/user/logout");
  },
};

// 视频相关 API
export const videoApi = {
  // 获取视频列表
  getVideos: (
    page = 1,
    limit = 10
  ): Promise<ApiResponse<{ videos: Video[]; total: number }>> => {
    return api.get(`/videos?page=${page}&limit=${limit}`);
  },

  // 获取单个视频详情
  getVideo: (id: number): Promise<ApiResponse<Video>> => {
    return api.get(`/videos/${id}`);
  },

  // 获取视频详细信息（包含任务步骤）
  getVideoDetail: (id: string): Promise<ApiResponse<VideoDetail>> => {
    return api.get(`/videos/${id}`);
  },

  // 获取视频文件列表
  getVideoFiles: (id: string): Promise<ApiResponse<VideoFile[]>> => {
    return api.get(`/videos/${id}/files`);
  },

  // 重试任务步骤
  retryTaskStep: (videoId: string, stepName: string): Promise<ApiResponse> => {
    return api.post(`/videos/${videoId}/steps/${stepName}/retry`);
  },

  // 重置所有失败的任务步骤
  resetAllFailedSteps: (
    videoId: string
  ): Promise<ApiResponse<{ reset_count: number; reset_steps: string[] }>> => {
    return api.post(`/videos/${videoId}/steps/reset-failed`);
  },

  // 重置所有任务步骤（不仅仅是失败的）
  resetAllSteps: (
    videoId: string
  ): Promise<ApiResponse<{ reset_count: number; reset_steps: string[] }>> => {
    return api.post(`/videos/${videoId}/steps/reset-all`);
  },

  // 提交新视频
  submitVideo: (data: VideoSubmissionRequest): Promise<ApiResponse<Video>> => {
    return api.post("/submit", data);
  },

  // 验证视频上传
  validateUpload: (videoId: string): Promise<ApiResponse<UploadValidation>> => {
    return api.post("/upload/validate", { video_id: videoId });
  },

  // 删除视频
  deleteVideo: (id: number): Promise<ApiResponse> => {
    return api.delete(`/videos/${id}`);
  },
};

// 字幕相关 API
export const subtitleApi = {
  // 获取视频字幕
  getSubtitles: (videoId: string): Promise<ApiResponse<any>> => {
    return api.get(`/subtitles/${videoId}`);
  },

  // 更新字幕
  updateSubtitles: (videoId: string, subtitles: any): Promise<ApiResponse> => {
    return api.put(`/subtitles/${videoId}`, { subtitles });
  },
};

// 会员相关 API
export const membershipApi = {
  // 获取会员信息
  getMembershipInfo: (): Promise<ApiResponse<MembershipInfo>> => {
    return api.get("/membership/info");
  },

  // 获取所有等级配置
  getAllTiers: (): Promise<ApiResponse<TierConfig[]>> => {
    return api.get("/membership/tiers");
  },

  // 获取配额信息
  getQuotaInfo: (): Promise<ApiResponse<QuotaInfo>> => {
    return api.get("/membership/quota");
  },

  // 获取可用功能列表
  getAvailableFeatures: (): Promise<ApiResponse<AvailableFeatures>> => {
    return api.get("/membership/features");
  },

  // 检查功能是否可用
  checkFeature: (feature: string): Promise<ApiResponse<FeatureCheckResult>> => {
    return api.get(`/membership/features/${feature}/check`);
  },

  // 获取加油包状态
  getBoostPackStatus: (): Promise<ApiResponse<BoostPackStatus>> => {
    return api.get("/membership/boost-pack");
  },

  // 购买加油包
  purchaseBoostPack: (
    data: PurchaseBoostPackRequest
  ): Promise<ApiResponse<PurchaseBoostPackResponse>> => {
    return api.post("/membership/boost-pack/purchase", data);
  },
};

export default api;
