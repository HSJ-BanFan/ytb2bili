"use client";

import { useState, useEffect, useCallback } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import { toolConfigApi, ToolAIConfig, ToolPreference } from '@/lib/toolConfig';
import {
    Settings, Bot, Sliders, Key, Save, RefreshCw,
    CheckCircle, AlertCircle, Eye, EyeOff, Sparkles
} from 'lucide-react';

export default function UserSettingsPage() {
    const [activeTab, setActiveTab] = useState<'ai' | 'preferences'>('ai');
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

    // AI 配置状态
    const [aiConfig, setAiConfig] = useState<ToolAIConfig | null>(null);
    const [aiLoading, setAiLoading] = useState(true);

    // 偏好设置状态
    const [preferences, setPreferences] = useState<ToolPreference | null>(null);
    const [prefLoading, setPrefLoading] = useState(true);

    // 显示/隐藏 API Key
    const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});

    // 加载 AI 配置
    const loadAIConfig = useCallback(async () => {
        setAiLoading(true);
        const config = await toolConfigApi.getAIConfig();
        setAiConfig(config);
        setAiLoading(false);
    }, []);

    // 加载偏好设置
    const loadPreferences = useCallback(async () => {
        setPrefLoading(true);
        const prefs = await toolConfigApi.getPreferences();
        setPreferences(prefs);
        setPrefLoading(false);
    }, []);

    useEffect(() => {
        loadAIConfig();
        loadPreferences();
    }, [loadAIConfig, loadPreferences]);

    // 保存 AI 配置
    const saveAIConfig = async () => {
        if (!aiConfig) return;
        setSaving(true);
        setMessage(null);
        const success = await toolConfigApi.updateAIConfig(aiConfig);
        setSaving(false);
        if (success) {
            setMessage({ type: 'success', text: 'AI 配置已保存' });
        } else {
            setMessage({ type: 'error', text: '保存失败，请重试' });
        }
        setTimeout(() => setMessage(null), 3000);
    };

    // 保存偏好设置
    const savePreferences = async () => {
        if (!preferences) return;
        setSaving(true);
        setMessage(null);
        const success = await toolConfigApi.updatePreferences(preferences);
        setSaving(false);
        if (success) {
            setMessage({ type: 'success', text: '偏好设置已保存' });
        } else {
            setMessage({ type: 'error', text: '保存失败，请重试' });
        }
        setTimeout(() => setMessage(null), 3000);
    };

    // 更新 AI 配置字段
    const updateAIField = (field: keyof ToolAIConfig, value: any) => {
        if (!aiConfig) return;
        setAiConfig({ ...aiConfig, [field]: value });
    };

    // 更新偏好设置字段
    const updatePrefField = (field: keyof ToolPreference, value: any) => {
        if (!preferences) return;
        setPreferences({ ...preferences, [field]: value });
    };

    // 切换 Key 显示
    const toggleShowKey = (key: string) => {
        setShowKeys(prev => ({ ...prev, [key]: !prev[key] }));
    };

    // 遮蔽 API Key
    const maskKey = (key: string, show: boolean) => {
        if (!key) return '';
        if (show) return key;
        if (key.length <= 8) return '••••••••';
        return key.slice(0, 4) + '••••••••' + key.slice(-4);
    };

    return (
        <AppLayout>
            <div className="space-y-6">
                {/* 页面标题 */}
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-2">
                            <Sparkles className="w-6 h-6 text-purple-500" />
                            工具设置
                        </h1>
                        <p className="text-gray-500 mt-1">配置全局 AI 服务和偏好</p>
                    </div>
                    <a
                        href="/settings"
                        className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition-colors text-sm flex items-center gap-2"
                    >
                        <Settings className="w-4 h-4" />
                        系统设置
                    </a>
                </div>

                {/* 消息提示 */}
                {message && (
                    <div className={`p-4 rounded-lg flex items-center gap-2 ${message.type === 'success'
                            ? 'bg-green-50 text-green-700 border border-green-200'
                            : 'bg-red-50 text-red-700 border border-red-200'
                        }`}>
                        {message.type === 'success' ? <CheckCircle className="w-5 h-5" /> : <AlertCircle className="w-5 h-5" />}
                        {message.text}
                    </div>
                )}

                {/* 标签页导航 */}
                <div className="bg-white rounded-lg shadow-md">
                    <div className="border-b border-gray-200">
                        <nav className="flex -mb-px">
                            <button
                                onClick={() => setActiveTab('ai')}
                                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${activeTab === 'ai'
                                        ? 'border-purple-500 text-purple-600'
                                        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                                    }`}
                            >
                                <Bot className="w-4 h-4 inline mr-2" />
                                AI 配置
                            </button>
                            <button
                                onClick={() => setActiveTab('preferences')}
                                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${activeTab === 'preferences'
                                        ? 'border-purple-500 text-purple-600'
                                        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                                    }`}
                            >
                                <Sliders className="w-4 h-4 inline mr-2" />
                                偏好设置
                            </button>
                        </nav>
                    </div>
                </div>

                {/* AI 配置标签页 */}
                {activeTab === 'ai' && (
                    <div className="bg-white rounded-lg shadow-md">
                        <div className="p-6 border-b border-gray-200">
                            <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                    <Key className="w-5 h-5 text-purple-600" />
                                    <h2 className="text-lg font-medium text-gray-900">AI API 密钥</h2>
                                </div>
                                <div className="flex gap-2">
                                    <button
                                        onClick={loadAIConfig}
                                        disabled={aiLoading}
                                        className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                                        title="刷新"
                                    >
                                        <RefreshCw className={`w-4 h-4 ${aiLoading ? 'animate-spin' : ''}`} />
                                    </button>
                                    <button
                                        onClick={saveAIConfig}
                                        disabled={saving || aiLoading}
                                        className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors flex items-center gap-2 disabled:opacity-50"
                                    >
                                        <Save className="w-4 h-4" />
                                        保存配置
                                    </button>
                                </div>
                            </div>
                            <p className="text-sm text-gray-500 mt-1">配置您自己的 API 密钥，优先于系统配置使用</p>
                        </div>

                        {aiLoading ? (
                            <div className="p-8 text-center">
                                <div className="inline-block w-6 h-6 border-2 border-purple-500 border-t-transparent rounded-full animate-spin"></div>
                                <p className="text-gray-500 mt-2">加载中...</p>
                            </div>
                        ) : aiConfig ? (
                            <div className="p-6 space-y-6">
                                {/* DeepSeek 配置 */}
                                <div className="border rounded-lg p-4">
                                    <div className="flex items-center justify-between mb-4">
                                        <h3 className="font-medium text-gray-900">DeepSeek</h3>
                                        <label className="flex items-center gap-2 cursor-pointer">
                                            <input
                                                type="checkbox"
                                                checked={aiConfig.deepseek_enabled}
                                                onChange={(e) => updateAIField('deepseek_enabled', e.target.checked)}
                                                className="w-4 h-4 text-purple-600 rounded"
                                            />
                                            <span className="text-sm text-gray-600">启用</span>
                                        </label>
                                    </div>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                                            <div className="relative">
                                                <input
                                                    type={showKeys['deepseek'] ? 'text' : 'password'}
                                                    value={aiConfig.deepseek_api_key || ''}
                                                    onChange={(e) => updateAIField('deepseek_api_key', e.target.value)}
                                                    placeholder="sk-..."
                                                    className="w-full p-2 border rounded-lg pr-10"
                                                />
                                                <button
                                                    type="button"
                                                    onClick={() => toggleShowKey('deepseek')}
                                                    className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                                                >
                                                    {showKeys['deepseek'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                                </button>
                                            </div>
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">模型</label>
                                            <input
                                                type="text"
                                                value={aiConfig.deepseek_model || 'deepseek-chat'}
                                                onChange={(e) => updateAIField('deepseek_model', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                    </div>
                                </div>

                                {/* Gemini 配置 */}
                                <div className="border rounded-lg p-4">
                                    <div className="flex items-center justify-between mb-4">
                                        <h3 className="font-medium text-gray-900">Google Gemini</h3>
                                        <label className="flex items-center gap-2 cursor-pointer">
                                            <input
                                                type="checkbox"
                                                checked={aiConfig.gemini_enabled}
                                                onChange={(e) => updateAIField('gemini_enabled', e.target.checked)}
                                                className="w-4 h-4 text-purple-600 rounded"
                                            />
                                            <span className="text-sm text-gray-600">启用</span>
                                        </label>
                                    </div>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                                            <div className="relative">
                                                <input
                                                    type={showKeys['gemini'] ? 'text' : 'password'}
                                                    value={aiConfig.gemini_api_key || ''}
                                                    onChange={(e) => updateAIField('gemini_api_key', e.target.value)}
                                                    placeholder="AIza..."
                                                    className="w-full p-2 border rounded-lg pr-10"
                                                />
                                                <button
                                                    type="button"
                                                    onClick={() => toggleShowKey('gemini')}
                                                    className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                                                >
                                                    {showKeys['gemini'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                                </button>
                                            </div>
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">模型</label>
                                            <input
                                                type="text"
                                                value={aiConfig.gemini_model || 'gemini-2.0-flash'}
                                                onChange={(e) => updateAIField('gemini_model', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                    </div>
                                </div>

                                {/* OpenAI Compatible 配置 */}
                                <div className="border rounded-lg p-4">
                                    <div className="flex items-center justify-between mb-4">
                                        <h3 className="font-medium text-gray-900">OpenAI Compatible</h3>
                                        <label className="flex items-center gap-2 cursor-pointer">
                                            <input
                                                type="checkbox"
                                                checked={aiConfig.openai_enabled}
                                                onChange={(e) => updateAIField('openai_enabled', e.target.checked)}
                                                className="w-4 h-4 text-purple-600 rounded"
                                            />
                                            <span className="text-sm text-gray-600">启用</span>
                                        </label>
                                    </div>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                                            <div className="relative">
                                                <input
                                                    type={showKeys['openai'] ? 'text' : 'password'}
                                                    value={aiConfig.openai_api_key || ''}
                                                    onChange={(e) => updateAIField('openai_api_key', e.target.value)}
                                                    placeholder="sk-..."
                                                    className="w-full p-2 border rounded-lg pr-10"
                                                />
                                                <button
                                                    type="button"
                                                    onClick={() => toggleShowKey('openai')}
                                                    className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                                                >
                                                    {showKeys['openai'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                                </button>
                                            </div>
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">Base URL</label>
                                            <input
                                                type="text"
                                                value={aiConfig.openai_base_url || 'https://api.openai.com/v1'}
                                                onChange={(e) => updateAIField('openai_base_url', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">模型</label>
                                            <input
                                                type="text"
                                                value={aiConfig.openai_model || 'gpt-3.5-turbo'}
                                                onChange={(e) => updateAIField('openai_model', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">提供商</label>
                                            <select
                                                value={aiConfig.openai_provider || 'openai'}
                                                onChange={(e) => updateAIField('openai_provider', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            >
                                                <option value="openai">OpenAI</option>
                                                <option value="deepseek">DeepSeek</option>
                                                <option value="qwen">通义千问</option>
                                                <option value="zhipu">智谱AI</option>
                                                <option value="custom">自定义</option>
                                            </select>
                                        </div>
                                    </div>
                                </div>

                                {/* 提示信息 */}
                                <div className="bg-purple-50 p-4 rounded-lg">
                                    <p className="text-sm text-purple-800">
                                        <strong>配置优先级：</strong>此处配置将优先于系统默认值使用。未配置的服务会自动使用系统默认配置。
                                    </p>
                                </div>
                            </div>
                        ) : (
                            <div className="p-8 text-center text-gray-500">
                                <AlertCircle className="w-12 h-12 mx-auto mb-4 text-gray-300" />
                                <p>无法加载配置，请刷新重试</p>
                            </div>
                        )}
                    </div>
                )}

                {/* 偏好设置标签页 */}
                {activeTab === 'preferences' && (
                    <div className="bg-white rounded-lg shadow-md">
                        <div className="p-6 border-b border-gray-200">
                            <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                    <Sliders className="w-5 h-5 text-purple-600" />
                                    <h2 className="text-lg font-medium text-gray-900">偏好设置</h2>
                                </div>
                                <div className="flex gap-2">
                                    <button
                                        onClick={loadPreferences}
                                        disabled={prefLoading}
                                        className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                                        title="刷新"
                                    >
                                        <RefreshCw className={`w-4 h-4 ${prefLoading ? 'animate-spin' : ''}`} />
                                    </button>
                                    <button
                                        onClick={savePreferences}
                                        disabled={saving || prefLoading}
                                        className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors flex items-center gap-2 disabled:opacity-50"
                                    >
                                        <Save className="w-4 h-4" />
                                        保存偏好
                                    </button>
                                </div>
                            </div>
                        </div>

                        {prefLoading ? (
                            <div className="p-8 text-center">
                                <div className="inline-block w-6 h-6 border-2 border-purple-500 border-t-transparent rounded-full animate-spin"></div>
                                <p className="text-gray-500 mt-2">加载中...</p>
                            </div>
                        ) : preferences ? (
                            <div className="p-6 space-y-6">
                                {/* 上传默认值 */}
                                <div className="border rounded-lg p-4">
                                    <h3 className="font-medium text-gray-900 mb-4">上传默认值</h3>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <label className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg">
                                            <input
                                                type="checkbox"
                                                checked={preferences.default_auto_upload}
                                                onChange={(e) => updatePrefField('default_auto_upload', e.target.checked)}
                                                className="w-4 h-4 text-purple-600 rounded"
                                            />
                                            <div>
                                                <span className="font-medium text-gray-700">默认自动上传</span>
                                                <p className="text-xs text-gray-500">新视频处理后自动上传到 B站</p>
                                            </div>
                                        </label>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">默认上传延迟 (分钟)</label>
                                            <input
                                                type="number"
                                                min="0"
                                                max="1440"
                                                value={preferences.default_upload_delay}
                                                onChange={(e) => updatePrefField('default_upload_delay', parseInt(e.target.value) || 0)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">默认字幕延迟 (分钟)</label>
                                            <input
                                                type="number"
                                                min="0"
                                                max="1440"
                                                value={preferences.default_subtitle_delay}
                                                onChange={(e) => updatePrefField('default_subtitle_delay', parseInt(e.target.value) || 0)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">默认分区 ID</label>
                                            <input
                                                type="number"
                                                value={preferences.default_tid}
                                                onChange={(e) => updatePrefField('default_tid', parseInt(e.target.value) || 122)}
                                                className="w-full p-2 border rounded-lg"
                                            />
                                        </div>
                                    </div>
                                </div>

                                {/* UI 偏好 */}
                                <div className="border rounded-lg p-4">
                                    <h3 className="font-medium text-gray-900 mb-4">界面偏好</h3>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">主题</label>
                                            <select
                                                value={preferences.theme}
                                                onChange={(e) => updatePrefField('theme', e.target.value)}
                                                className="w-full p-2 border rounded-lg"
                                            >
                                                <option value="light">浅色</option>
                                                <option value="dark">深色</option>
                                                <option value="system">跟随系统</option>
                                            </select>
                                        </div>
                                        <div>
                                            <label className="block text-sm font-medium text-gray-700 mb-1">每页显示数量</label>
                                            <select
                                                value={preferences.items_per_page}
                                                onChange={(e) => updatePrefField('items_per_page', parseInt(e.target.value))}
                                                className="w-full p-2 border rounded-lg"
                                            >
                                                <option value={10}>10</option>
                                                <option value={20}>20</option>
                                                <option value={50}>50</option>
                                                <option value={100}>100</option>
                                            </select>
                                        </div>
                                    </div>
                                </div>

                            </div>
                        ) : (
                            <div className="p-8 text-center text-gray-500">
                                <AlertCircle className="w-12 h-12 mx-auto mb-4 text-gray-300" />
                                <p>无法加载偏好设置，请刷新重试</p>
                            </div>
                        )}
                    </div>
                )}
            </div>
        </AppLayout>
    );
}
