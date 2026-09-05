"use client";

import { useState } from "react";
import AppLayout from "@/components/layout/AppLayout";
import {
  Network,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Plus,
  Play,
  Pause,
  Tv,
  CheckCircle2,
  Clock,
  ExternalLink,
  AlertTriangle,
  UserCheck,
  Video,
} from "lucide-react";

// 来源频道类型
interface SourceChannelItem {
  id: number;
  platform: "youtube" | "twitch";
  channelId: string;
  channelName: string;
  channelUrl: string;
  fetchType: "channel_video" | "live_stream" | "playlist";
  isEnabled: boolean;
  status: "active" | "paused" | "error";
  cronExpression: string;
  lastCheckedAt: string;
  rulesCount: number;
}

// 策略矩阵规则
interface StrategyRuleItem {
  id: number;
  sourceChannelId: number;
  sourceChannelName: string;
  biliAccountId: number;
  biliAccountName: string;
  biliMid: number;
  ruleName: string;
  isEnabled: boolean;
  priority: number;
  autoPublish: boolean; // true=全自动, false=人工审核
  dynamicTitleTemplate: string;
  descTemplate: string;
  defaultTags: string[];
  categoryId: number; // 分区 TID
  categoryName: string;
  delayMinutes: number;
}

// 投稿防线熔断状态
interface GuardrailStatus {
  scope: "global" | "channel" | "account";
  targetId: string;
  targetName: string;
  isPaused: boolean;
  pauseReason?: string;
  consecutiveFailures: number;
  failureThreshold: number;
  autoResumeAt?: string;
}

export default function StrategyMatrixPage() {
  // --- 模拟状态与响应式 Hook ---
  const [globalPaused, setGlobalPaused] = useState(false);
  const [globalReason, setGlobalReason] = useState("");

  const [channels, setChannels] = useState<SourceChannelItem[]>([
    {
      id: 1,
      platform: "youtube",
      channelId: "UC_Tech_001",
      channelName: "Two Minute Papers (AI前沿)",
      channelUrl: "https://youtube.com/@twominutepapers",
      fetchType: "channel_video",
      isEnabled: true,
      status: "active",
      cronExpression: "@every 30m",
      lastCheckedAt: "10分钟前",
      rulesCount: 2,
    },
    {
      id: 2,
      platform: "youtube",
      channelId: "UC_Gaming_002",
      channelName: "IGN Official (游戏速递)",
      channelUrl: "https://youtube.com/@ign",
      fetchType: "channel_video",
      isEnabled: true,
      status: "active",
      cronExpression: "@every 1h",
      lastCheckedAt: "25分钟前",
      rulesCount: 1,
    },
    {
      id: 3,
      platform: "twitch",
      channelId: "shroud",
      channelName: "shroud (Twitch 高能直播)",
      channelUrl: "https://twitch.tv/shroud",
      fetchType: "live_stream",
      isEnabled: false,
      status: "paused",
      cronExpression: "@every 5m",
      lastCheckedAt: "1小时前",
      rulesCount: 1,
    },
  ]);

  const [rules, setRules] = useState<StrategyRuleItem[]>([
    {
      id: 101,
      sourceChannelId: 1,
      sourceChannelName: "Two Minute Papers (AI前沿)",
      biliAccountId: 1,
      biliAccountName: "极客科技观察室 (主号)",
      biliMid: 18492011,
      ruleName: "AI 论文精翻全自动发布",
      isEnabled: true,
      priority: 10,
      autoPublish: true,
      dynamicTitleTemplate: "【中字精翻】{ai_title} | AI每日速递",
      descTemplate: "原作者: Two Minute Papers\n由 Pi Agent 协同翻译与分段总结。\n原视频: {source_url}",
      defaultTags: ["人工智能", "机器学习", "科技前沿", "计算机视觉"],
      categoryId: 188,
      categoryName: "科技 - 数码",
      delayMinutes: 0,
    },
    {
      id: 102,
      sourceChannelId: 1,
      sourceChannelName: "Two Minute Papers (AI前沿)",
      biliAccountId: 2,
      biliAccountName: "学术搬运存档库 (副号)",
      biliMid: 49201992,
      ruleName: "副号备份待人工审",
      isEnabled: true,
      priority: 5,
      autoPublish: false,
      dynamicTitleTemplate: "【备份存档】{title}",
      descTemplate: "仅作学习研究备份。",
      defaultTags: ["科技", "存档"],
      categoryId: 188,
      categoryName: "科技 - 软件应用",
      delayMinutes: 60,
    },
    {
      id: 103,
      sourceChannelId: 2,
      sourceChannelName: "IGN Official (游戏速递)",
      biliAccountId: 3,
      biliAccountName: "主机游戏集锦站",
      biliMid: 66019283,
      ruleName: "IGN 预告片全自动搬运",
      isEnabled: true,
      priority: 8,
      autoPublish: true,
      dynamicTitleTemplate: "【IGN新游】{title}",
      descTemplate: "IGN 官方预告搬运，中文双语字幕。",
      defaultTags: ["单机游戏", "预告片", "Steam", "PS5"],
      categoryId: 17,
      categoryName: "单机游戏",
      delayMinutes: 0,
    },
  ]);

  const [accountGuardrails, setAccountGuardrails] = useState<GuardrailStatus[]>([
    {
      scope: "account",
      targetId: "1",
      targetName: "极客科技观察室 (主号)",
      isPaused: false,
      consecutiveFailures: 0,
      failureThreshold: 3,
    },
    {
      scope: "account",
      targetId: "2",
      targetName: "学术搬运存档库 (副号)",
      isPaused: true,
      pauseReason: "HTTP 601 (上传过快触发自动熔断保护)",
      consecutiveFailures: 3,
      failureThreshold: 3,
      autoResumeAt: "18 分钟后",
    },
  ]);

  // 模态弹窗状态
  const [showChannelModal, setShowChannelModal] = useState(false);
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [activeTab, setActiveTab] = useState<"matrix" | "channels" | "guardrails">("matrix");

  // 切换频道启用状态
  const toggleChannel = (id: number) => {
    setChannels((prev) =>
      prev.map((c) =>
        c.id === id
          ? {
              ...c,
              isEnabled: !c.isEnabled,
              status: !c.isEnabled ? "active" : "paused",
            }
          : c
      )
    );
  };

  // 切换规则状态
  const toggleRule = (id: number) => {
    setRules((prev) =>
      prev.map((r) => (r.id === id ? { ...r, isEnabled: !r.isEnabled } : r))
    );
  };

  // 切换自动发布 vs 待人工审
  const toggleRuleAutoPublish = (id: number) => {
    setRules((prev) =>
      prev.map((r) => (r.id === id ? { ...r, autoPublish: !r.autoPublish } : r))
    );
  };

  // 恢复账号熔断
  const resetAccountGuardrail = (targetId: string) => {
    setAccountGuardrails((prev) =>
      prev.map((g) =>
        g.targetId === targetId
          ? {
              ...g,
              isPaused: false,
              pauseReason: undefined,
              consecutiveFailures: 0,
              autoResumeAt: undefined,
            }
          : g
      )
    );
  };

  return (
    <AppLayout>
      <div className="space-y-6">
        {/* --- 顶部页头与急停总开关 --- */}
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
            <div>
              <div className="flex items-center space-x-3">
                <div className="p-2 bg-blue-50 text-blue-600 rounded-lg">
                  <Network className="w-6 h-6" />
                </div>
                <div>
                  <h1 className="text-xl font-bold text-gray-900">
                    搬运策略矩阵与投稿防线
                  </h1>
                  <p className="text-sm text-gray-500 mt-0.5">
                    多对多灵活分发网络、确定性幂等防重与三级熔断急停防线
                  </p>
                </div>
              </div>
            </div>

            {/* 全局急停防线控制 (Tier 1 Kill Switch) */}
            <div className="flex items-center space-x-3 bg-gray-50 border border-gray-200 px-4 py-2.5 rounded-lg">
              <div className="flex items-center space-x-2">
                {globalPaused ? (
                  <ShieldAlert className="w-5 h-5 text-red-600 animate-pulse" />
                ) : (
                  <ShieldCheck className="w-5 h-5 text-green-600" />
                )}
                <div>
                  <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Tier 1 全局急停防线
                  </div>
                  <div
                    className={`text-sm font-medium ${
                      globalPaused ? "text-red-600" : "text-green-700"
                    }`}
                  >
                    {globalPaused ? "已全面紧急拉闸" : "正常放行运行中"}
                  </div>
                </div>
              </div>

              <button
                onClick={() => {
                  const next = !globalPaused;
                  setGlobalPaused(next);
                  setGlobalReason(next ? "人工手动全局紧急切断" : "");
                }}
                className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                  globalPaused
                    ? "bg-green-600 text-white hover:bg-green-700"
                    : "bg-red-600 text-white hover:bg-red-700"
                }`}
              >
                {globalPaused ? "解除急停" : "紧急拉闸"}
              </button>
            </div>
          </div>

          {/* 全局拉闸告警条 */}
          {globalPaused && (
            <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-md flex items-center space-x-2 text-sm text-red-700">
              <AlertTriangle className="w-4 h-4 flex-shrink-0" />
              <span>
                <strong>系统已处于全局阻断状态：</strong>
                {globalReason ? ` [${globalReason}] ` : ""}所有的后台 biliup 投稿与任务步进均已被强制挂起，防线将自动保护账号免受外部风险扩散。
              </span>
            </div>
          )}

          {/* 导航标签卡 */}
          <div className="flex border-b border-gray-200 mt-6 -mb-6">
            <button
              onClick={() => setActiveTab("matrix")}
              className={`py-3 px-4 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "matrix"
                  ? "border-blue-600 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              策略矩阵规则网格 ({rules.length})
            </button>
            <button
              onClick={() => setActiveTab("channels")}
              className={`py-3 px-4 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "channels"
                  ? "border-blue-600 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              来源频道配置 ({channels.length})
            </button>
            <button
              onClick={() => setActiveTab("guardrails")}
              className={`py-3 px-4 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "guardrails"
                  ? "border-blue-600 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              三级熔断防线监控 ({accountGuardrails.filter((a) => a.isPaused).length} 处熔断)
            </button>
          </div>
        </div>

        {/* --- Tab 1: 策略矩阵规则网格 (多对多规则) --- */}
        {activeTab === "matrix" && (
          <div className="bg-white rounded-lg shadow-sm border border-gray-200">
            <div className="p-4 border-b border-gray-200 flex items-center justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  多对多搬运策略路由
                </h2>
                <p className="text-xs text-gray-500">
                  来源频道媒体被抓取后，将根据匹配规则独立生成任务，投往对应的 B 站目标账号
                </p>
              </div>
              <button
                onClick={() => setShowRuleModal(true)}
                className="flex items-center space-x-1.5 px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 transition"
              >
                <Plus className="w-4 h-4" />
                <span>新建策略规则</span>
              </button>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-sm text-left">
                <thead className="bg-gray-50 text-gray-500 font-medium text-xs border-b">
                  <tr>
                    <th className="py-3 px-4">规则名称 / 优先级</th>
                    <th className="py-3 px-4">来源频道 (Source)</th>
                    <th className="py-3 px-4">目标账号 (Target)</th>
                    <th className="py-3 px-4">发布模式 (Guardrail)</th>
                    <th className="py-3 px-4">B站分区 / 标签偏好</th>
                    <th className="py-3 px-4">规则开关</th>
                    <th className="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {rules.map((rule) => (
                    <tr key={rule.id} className="hover:bg-gray-50 transition">
                      <td className="py-3 px-4">
                        <div className="font-medium text-gray-900">
                          {rule.ruleName}
                        </div>
                        <div className="flex items-center space-x-2 mt-0.5">
                          <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded font-mono">
                            Priority: {rule.priority}
                          </span>
                          {rule.delayMinutes > 0 && (
                            <span className="text-xs text-amber-700 bg-amber-50 px-1.5 py-0.5 rounded flex items-center">
                              <Clock className="w-3 h-3 mr-0.5" />
                              延时 {rule.delayMinutes}m
                            </span>
                          )}
                        </div>
                      </td>

                      <td className="py-3 px-4">
                        <div className="flex items-center space-x-1.5 font-medium text-gray-800">
                          <Video className="w-4 h-4 text-red-600 flex-shrink-0" />
                          <span>{rule.sourceChannelName}</span>
                        </div>
                      </td>

                      <td className="py-3 px-4">
                        <div className="flex items-center space-x-1.5 font-medium text-blue-700">
                          <UserCheck className="w-4 h-4 text-blue-600 flex-shrink-0" />
                          <span>{rule.biliAccountName}</span>
                        </div>
                        <div className="text-xs text-gray-400 font-mono">
                          MID: {rule.biliMid}
                        </div>
                      </td>

                      <td className="py-3 px-4">
                        <button
                          onClick={() => toggleRuleAutoPublish(rule.id)}
                          className={`inline-flex items-center space-x-1 px-2.5 py-1 rounded-full text-xs font-medium cursor-pointer transition ${
                            rule.autoPublish
                              ? "bg-green-100 text-green-800 hover:bg-green-200"
                              : "bg-amber-100 text-amber-800 hover:bg-amber-200"
                          }`}
                          title="点击切换全自动投稿或人工确认"
                        >
                          {rule.autoPublish ? (
                            <>
                              <CheckCircle2 className="w-3.5 h-3.5 text-green-600" />
                              <span>全自动直投</span>
                            </>
                          ) : (
                            <>
                              <Clock className="w-3.5 h-3.5 text-amber-600" />
                              <span>需人工审核</span>
                            </>
                          )}
                        </button>
                      </td>

                      <td className="py-3 px-4">
                        <div className="text-xs text-gray-700 font-medium">
                          {rule.categoryName} (TID: {rule.categoryId})
                        </div>
                        <div className="flex flex-wrap gap-1 mt-1">
                          {rule.defaultTags.slice(0, 3).map((tag, idx) => (
                            <span
                              key={idx}
                              className="text-[11px] bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded"
                            >
                              #{tag}
                            </span>
                          ))}
                        </div>
                      </td>

                      <td className="py-3 px-4">
                        <button
                          onClick={() => toggleRule(rule.id)}
                          className={`w-10 h-5 flex items-center rounded-full p-1 duration-200 cursor-pointer ${
                            rule.isEnabled ? "bg-blue-600" : "bg-gray-300"
                          }`}
                        >
                          <div
                            className={`bg-white w-3.5 h-3.5 rounded-full shadow-md transform duration-200 ${
                              rule.isEnabled ? "translate-x-5" : "translate-x-0"
                            }`}
                          />
                        </button>
                      </td>

                      <td className="py-3 px-4 text-right">
                        <button className="text-xs text-blue-600 hover:text-blue-800 font-medium mr-3">
                          配置模版
                        </button>
                        <button className="text-xs text-red-500 hover:text-red-700 font-medium">
                          删除
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* --- Tab 2: 来源频道配置 --- */}
        {activeTab === "channels" && (
          <div className="bg-white rounded-lg shadow-sm border border-gray-200">
            <div className="p-4 border-b border-gray-200 flex items-center justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  来源媒体频道管理 (Tier 2 防线)
                </h2>
                <p className="text-xs text-gray-500">
                  定时轮询监听 YouTube / Twitch 频道；可对单一频道进行暂停与熔断隔离
                </p>
              </div>
              <button
                onClick={() => setShowChannelModal(true)}
                className="flex items-center space-x-1.5 px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 transition"
              >
                <Plus className="w-4 h-4" />
                <span>添加监控频道</span>
              </button>
            </div>

            <div className="divide-y divide-gray-200">
              {channels.map((channel) => (
                <div
                  key={channel.id}
                  className="p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 hover:bg-gray-50 transition"
                >
                  <div className="flex items-start space-x-3">
                    <div className="p-2.5 bg-gray-100 rounded-lg text-gray-700 flex-shrink-0 mt-0.5">
                      {channel.platform === "youtube" ? (
                        <Video className="w-5 h-5 text-red-600" />
                      ) : (
                        <Tv className="w-5 h-5 text-purple-600" />
                      )}
                    </div>
                    <div>
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-gray-900">
                          {channel.channelName}
                        </span>
                        <a
                          href={channel.channelUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="text-gray-400 hover:text-blue-600"
                        >
                          <ExternalLink className="w-3.5 h-3.5" />
                        </a>
                      </div>
                      <div className="flex items-center space-x-3 text-xs text-gray-500 mt-1">
                        <span className="font-mono">{channel.channelId}</span>
                        <span>•</span>
                        <span className="bg-gray-100 text-gray-700 px-1.5 py-0.5 rounded">
                          {channel.fetchType === "channel_video"
                            ? "点播视频搬运"
                            : "直播录制会话"}
                        </span>
                        <span>•</span>
                        <span>轮询: {channel.cronExpression}</span>
                        <span>•</span>
                        <span>上次检查: {channel.lastCheckedAt}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center space-x-4">
                    <span className="text-xs text-gray-600">
                      绑定规则: <strong>{channel.rulesCount}</strong> 条
                    </span>

                    <button
                      onClick={() => toggleChannel(channel.id)}
                      className={`px-3 py-1 text-xs font-medium rounded flex items-center space-x-1 transition ${
                        channel.isEnabled
                          ? "bg-amber-100 text-amber-800 hover:bg-amber-200"
                          : "bg-green-100 text-green-800 hover:bg-green-200"
                      }`}
                    >
                      {channel.isEnabled ? (
                        <>
                          <Pause className="w-3 h-3" />
                          <span>暂停采集</span>
                        </>
                      ) : (
                        <>
                          <Play className="w-3 h-3" />
                          <span>恢复采集</span>
                        </>
                      )}
                    </button>

                    <button className="text-xs text-blue-600 hover:text-blue-800">
                      立即抓取
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* --- Tab 3: 三级熔断防线监控 (Tier 3 Account Guardrails) --- */}
        {activeTab === "guardrails" && (
          <div className="space-y-4">
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
              <h2 className="text-base font-semibold text-gray-900">
                Tier 3 账号风控与 HTTP 601 限流熔断器
              </h2>
              <p className="text-xs text-gray-500 mt-0.5">
                当 Bilibili 返回 code 601（“您上传视频过快”）或凭证失效时，系统自动对该账号触发熔断保护，进入 30 分钟冷却隔离，防止批量上传风控扩散。
              </p>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
                {accountGuardrails.map((guardrail) => (
                  <div
                    key={guardrail.targetId}
                    className={`p-4 rounded-lg border ${
                      guardrail.isPaused
                        ? "bg-red-50 border-red-200"
                        : "bg-green-50 border-green-200"
                    }`}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex items-center space-x-2">
                        {guardrail.isPaused ? (
                          <ShieldAlert className="w-5 h-5 text-red-600" />
                        ) : (
                          <ShieldCheck className="w-5 h-5 text-green-600" />
                        )}
                        <div>
                          <div className="font-semibold text-gray-900 text-sm">
                            {guardrail.targetName}
                          </div>
                          <div className="text-xs text-gray-500">
                            账号 ID: {guardrail.targetId}
                          </div>
                        </div>
                      </div>

                      <span
                        className={`text-xs px-2 py-0.5 rounded font-medium ${
                          guardrail.isPaused
                            ? "bg-red-200 text-red-800"
                            : "bg-green-200 text-green-800"
                        }`}
                      >
                        {guardrail.isPaused ? "已熔断拦截" : "健康放行"}
                      </span>
                    </div>

                    {guardrail.isPaused && (
                      <div className="mt-3 text-xs text-red-700 space-y-1">
                        <div>
                          <strong>熔断原因：</strong> {guardrail.pauseReason}
                        </div>
                        <div>
                          <strong>连续失败：</strong> {guardrail.consecutiveFailures} / {guardrail.failureThreshold} 次阈值
                        </div>
                        {guardrail.autoResumeAt && (
                          <div>
                            <strong>自动解封：</strong> 预计将在 {guardrail.autoResumeAt} 结束冷却并放行
                          </div>
                        )}
                      </div>
                    )}

                    <div className="mt-4 pt-3 border-t border-gray-200/50 flex justify-end">
                      {guardrail.isPaused ? (
                        <button
                          onClick={() => resetAccountGuardrail(guardrail.targetId)}
                          className="px-3 py-1 bg-red-600 text-white rounded text-xs font-medium hover:bg-red-700 transition"
                        >
                          手动重置并恢复
                        </button>
                      ) : (
                        <span className="text-xs text-gray-400">
                          连续失败计数: 0 (正常)
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* 幂等防重状态统计 */}
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-900 mb-2 flex items-center space-x-2">
                <Shield className="w-4 h-4 text-blue-600" />
                <span>物理幂等防重队列 (cw_publish_fingerprints)</span>
              </h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
                <div className="p-3 bg-gray-50 rounded-lg">
                  <div className="text-xl font-bold text-gray-900">128</div>
                  <div className="text-xs text-gray-500">已投稿指纹 (Published)</div>
                </div>
                <div className="p-3 bg-blue-50 rounded-lg">
                  <div className="text-xl font-bold text-blue-700">3</div>
                  <div className="text-xs text-blue-600">正在执行锁 (Locked)</div>
                </div>
                <div className="p-3 bg-amber-50 rounded-lg">
                  <div className="text-xl font-bold text-amber-700">6</div>
                  <div className="text-xs text-amber-600">待处理排队 (Pending)</div>
                </div>
                <div className="p-3 bg-red-50 rounded-lg">
                  <div className="text-xl font-bold text-red-700">0</div>
                  <div className="text-xs text-red-600">死信隔离 (DeadLetter)</div>
                </div>
              </div>
            </div>
          </div>
        )}

          {/* --- 简易弹窗模拟 (添加频道) --- */}
          {showChannelModal && (
            <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
              <div className="bg-white rounded-lg max-w-md w-full p-6 space-y-4 shadow-xl">
                <h3 className="text-lg font-bold text-gray-900">
                  添加来源监控频道
                </h3>
                <div className="space-y-3 text-sm">
                  <div>
                    <label className="block font-medium text-gray-700 mb-1">
                      来源平台
                    </label>
                    <select className="w-full border border-gray-300 rounded p-2 text-sm">
                      <option value="youtube">YouTube</option>
                      <option value="twitch">Twitch</option>
                    </select>
                  </div>
                  <div>
                    <label className="block font-medium text-gray-700 mb-1">
                      频道原生 ID 或 URL
                    </label>
                    <input
                      type="text"
                      placeholder="UCxxxxxxxx 或 https://..."
                      className="w-full border border-gray-300 rounded p-2 text-sm"
                    />
                  </div>
                  <div>
                    <label className="block font-medium text-gray-700 mb-1">
                      采集模式
                    </label>
                    <select className="w-full border border-gray-300 rounded p-2 text-sm">
                      <option value="channel_video">点播视频搬运 (Channel Video)</option>
                      <option value="live_stream">直播实时录制 (Live Stream)</option>
                    </select>
                  </div>
                </div>
                <div className="flex justify-end space-x-3 pt-4 border-t">
                  <button
                    onClick={() => setShowChannelModal(false)}
                    className="px-4 py-2 border rounded text-gray-600 hover:bg-gray-50 text-sm"
                  >
                    取消
                  </button>
                  <button
                    onClick={() => setShowChannelModal(false)}
                    className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
                  >
                    确认添加
                  </button>
                </div>
              </div>
            </div>
          )}
        {showRuleModal && (
          <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
            <div className="bg-white rounded-lg max-w-lg w-full p-6 space-y-4 shadow-xl">
              <h3 className="text-lg font-bold text-gray-900">
                新建搬运策略路由规则
              </h3>
              <div className="space-y-3 text-sm">
                <div>
                  <label className="block font-medium text-gray-700 mb-1">
                    选择来源频道
                  </label>
                  <select className="w-full border border-gray-300 rounded p-2 text-sm">
                    {channels.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.channelName} ({c.platform})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block font-medium text-gray-700 mb-1">
                    目标 B 站投稿账号
                  </label>
                  <select className="w-full border border-gray-300 rounded p-2 text-sm">
                    <option>极客科技观察室 (主号 - 18492011)</option>
                    <option>学术搬运存档库 (副号 - 49201992)</option>
                    <option>主机游戏集锦站 (66019283)</option>
                  </select>
                </div>
                <div>
                  <label className="block font-medium text-gray-700 mb-1">
                    规则名称
                  </label>
                  <input
                    type="text"
                    defaultValue="自动精翻搬运"
                    className="w-full border border-gray-300 rounded p-2 text-sm"
                  />
                </div>
                <div>
                  <label className="block font-medium text-gray-700 mb-1">
                    发布策略
                  </label>
                  <div className="flex items-center space-x-4">
                    <label className="flex items-center space-x-1.5 cursor-pointer">
                      <input type="radio" name="publish_mode" defaultChecked />
                      <span>全自动立即投稿</span>
                    </label>
                    <label className="flex items-center space-x-1.5 cursor-pointer">
                      <input type="radio" name="publish_mode" />
                      <span>生成草稿待人工审核</span>
                    </label>
                  </div>
                </div>
              </div>
              <div className="flex justify-end space-x-3 pt-4 border-t">
                <button
                  onClick={() => setShowRuleModal(false)}
                  className="px-4 py-2 border rounded text-gray-600 hover:bg-gray-50 text-sm"
                >
                  取消
                </button>
                <button
                  onClick={() => setShowRuleModal(false)}
                  className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
                >
                  保存并生效
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  );
}
