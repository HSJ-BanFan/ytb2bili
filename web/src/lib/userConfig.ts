/**
 * 用户配置 API 封装
 * 用于管理用户个人 AI 配置和偏好设置
 */

// 获取 API 基础 URL
const getApiBaseUrl = () => {
    if (typeof window !== 'undefined') {
        const { protocol, hostname, port } = window.location;
        return `${protocol}//${hostname}${port ? ':' + port : ''}`;
    }
    return 'http://localhost:8096';
};

// 获取认证头
const getAuthHeaders = () => {
    const token = localStorage.getItem('jwt_token');
    return {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
    };
};

// 用户 AI 配置类型
export interface UserAIConfig {
    id?: number;
    user_id?: number;
    // DeepSeek
    deepseek_enabled: boolean;
    deepseek_api_key: string;
    deepseek_model: string;
    // Gemini
    gemini_enabled: boolean;
    gemini_api_key: string;
    gemini_api_keys: string;
    gemini_model: string;
    gemini_timeout: number;
    gemini_max_tokens: number;
    // OpenAI Compatible
    openai_enabled: boolean;
    openai_api_key: string;
    openai_base_url: string;
    openai_model: string;
    openai_provider: string;
    openai_timeout: number;
    openai_max_tokens: number;
    // Baidu
    baidu_enabled: boolean;
    baidu_app_id: string;
    baidu_secret: string;
}

// 用户偏好类型
export interface UserPreference {
    id?: number;
    user_id?: number;
    // 上传默认值
    default_auto_upload: boolean;
    default_upload_delay: number;
    default_subtitle_delay: number;
    default_copyright: number;
    default_source: string;
    default_tid: number;
    // UI 偏好
    theme: string;
    language: string;
    items_per_page: number;
    show_advanced: boolean;
    // 通知
    email_notifications_enabled: boolean;
    enable_analytics: boolean;
}

// API 响应类型
interface ApiResponse<T> {
    code: number;
    message: string;
    data?: T;
}

/**
 * 用户配置 API
 */
export const userConfigApi = {
    // ==================== AI 配置 ====================

    /**
     * 获取用户 AI 配置
     */
    async getAIConfig(): Promise<UserAIConfig | null> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/user/config/ai`, {
                headers: getAuthHeaders(),
            });
            const data: ApiResponse<UserAIConfig> = await res.json();
            if (data.code === 0 && data.data) {
                return data.data;
            }
            return null;
        } catch (e) {
            console.error('获取用户 AI 配置失败:', e);
            return null;
        }
    },

    /**
     * 更新用户 AI 配置
     */
    async updateAIConfig(config: Partial<UserAIConfig>): Promise<boolean> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/user/config/ai`, {
                method: 'PUT',
                headers: getAuthHeaders(),
                body: JSON.stringify(config),
            });
            const data: ApiResponse<UserAIConfig> = await res.json();
            return data.code === 0;
        } catch (e) {
            console.error('更新用户 AI 配置失败:', e);
            return false;
        }
    },

    // ==================== 偏好设置 ====================

    /**
     * 获取用户偏好
     */
    async getPreferences(): Promise<UserPreference | null> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/user/config/preferences`, {
                headers: getAuthHeaders(),
            });
            const data: ApiResponse<UserPreference> = await res.json();
            if (data.code === 0 && data.data) {
                return data.data;
            }
            return null;
        } catch (e) {
            console.error('获取用户偏好失败:', e);
            return null;
        }
    },

    /**
     * 更新用户偏好
     */
    async updatePreferences(prefs: Partial<UserPreference>): Promise<boolean> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/user/config/preferences`, {
                method: 'PUT',
                headers: getAuthHeaders(),
                body: JSON.stringify(prefs),
            });
            const data: ApiResponse<UserPreference> = await res.json();
            return data.code === 0;
        } catch (e) {
            console.error('更新用户偏好失败:', e);
            return false;
        }
    },
};

export default userConfigApi;
