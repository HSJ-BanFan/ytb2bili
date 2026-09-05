/**
 * 全局工具配置 API 封装
 */

// 获取 API 基础 URL
const getApiBaseUrl = () => {
    if (typeof window !== 'undefined') {
        const { protocol, hostname, port } = window.location;
        return `${protocol}//${hostname}${port ? ':' + port : ''}`;
    }
    return 'http://localhost:8096';
};

const getRequestHeaders = (): HeadersInit => ({ 'Content-Type': 'application/json' });

// 工具 AI 配置类型
export interface ToolAIConfig {
    id?: number;
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

// 全局偏好类型
export interface ToolPreference {
    id?: number;
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
    enable_analytics: boolean;
}

// API 响应类型
interface ApiResponse<T> {
    code: number;
    message: string;
    data?: T;
}

/**
 * 工具配置 API
 */
export const toolConfigApi = {
    // ==================== AI 配置 ====================

    /**
     * 获取工具 AI 配置
     */
    async getAIConfig(): Promise<ToolAIConfig | null> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/tool/config/ai`, {
                headers: getRequestHeaders(),
            });
            const data: ApiResponse<ToolAIConfig> = await res.json();
            if (data.code === 0 && data.data) {
                return data.data;
            }
            return null;
        } catch (e) {
            console.error('获取工具 AI 配置失败:', e);
            return null;
        }
    },

    /**
     * 更新工具 AI 配置
     */
    async updateAIConfig(config: Partial<ToolAIConfig>): Promise<boolean> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/tool/config/ai`, {
                method: 'PUT',
                headers: getRequestHeaders(),
                body: JSON.stringify(config),
            });
            const data: ApiResponse<ToolAIConfig> = await res.json();
            return data.code === 0;
        } catch (e) {
            console.error('更新工具 AI 配置失败:', e);
            return false;
        }
    },

    // ==================== 偏好设置 ====================

    /**
     * 获取工具偏好
     */
    async getPreferences(): Promise<ToolPreference | null> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/tool/config/preferences`, {
                headers: getRequestHeaders(),
            });
            const data: ApiResponse<ToolPreference> = await res.json();
            if (data.code === 0 && data.data) {
                return data.data;
            }
            return null;
        } catch (e) {
            console.error('获取工具偏好失败:', e);
            return null;
        }
    },

    /**
     * 更新工具偏好
     */
    async updatePreferences(prefs: Partial<ToolPreference>): Promise<boolean> {
        try {
            const res = await fetch(`${getApiBaseUrl()}/api/v1/tool/config/preferences`, {
                method: 'PUT',
                headers: getRequestHeaders(),
                body: JSON.stringify(prefs),
            });
            const data: ApiResponse<ToolPreference> = await res.json();
            return data.code === 0;
        } catch (e) {
            console.error('更新工具偏好失败:', e);
            return false;
        }
    },
};

export default toolConfigApi;
