"use client";

import { useState } from "react";
import AppLayout from "@/components/layout/AppLayout";
import {
  Bot,
  CheckCircle2,
  Clock,
  AlertTriangle,
  Sparkles,
  Search,
  Send,
  RotateCcw,
  FileText,
  Layers,
  Video,
  Check,
  X,
  AlertCircle,
} from "lucide-react";

interface DecisionTraceItem {
  id: string;
  sourceVideoId: string;
  sourceTitle: string;
  sourcePlatform: "youtube" | "twitch";
  sourceChannel: string;
  duration: string;
  whisperSummary: string;
  targetBiliAccount: string;
  biliMid: number;
  matchedRuleName: string;
  status: "pending_review" | "auto_approved" | "published" | "deadletter";
  createdAt: string;
  llmModel: string;
  executionLatency: string;
  tokenUsage: number;
  reasoningChain: string[];
  candidateTitles: string[];
  selectedTitle: string;
  description: string;
  tags: string[];
  segments: { part: number; title: string; duration: string }[];
  deadLetterReason?: string;
  biliBvid?: string;
}

export default function AgentDecisionTracePage() {
  const [filterStatus, setFilterStatus] = useState<string>("all");
  const [searchQuery, setSearchQuery] = useState("");

  const [traces, setTraces] = useState<DecisionTraceItem[]>([
    {
      id: "trace_001",
      sourceVideoId: "dQw4w9WgXcQ",
      sourceTitle: "OpenAI Just Solved Robotics In 5 Minutes! Full Analysis",
      sourcePlatform: "youtube",
      sourceChannel: "Two Minute Papers (AI前沿)",
      duration: "08:42",
      whisperSummary:
        "视频探讨了最新多模态强化学习在机械臂末端执行器中的泛化应用。作者展示了如何通过自监督预训练实现零样本重定向，大幅缩短模型训练周期。",
      targetBiliAccount: "学术搬运存档库 (副号)",
      biliMid: 49201992,
      matchedRuleName: "副号备份待人工审",
      status: "pending_review",
      createdAt: "15 分钟前",
      llmModel: "pi-agent (gpt-4o)",
      executionLatency: "2.1s",
      tokenUsage: 1640,
      reasoningChain: [
        "原标题含有典型英文 Clickbait 夸张短语 'In 5 Minutes'，需要做本土化客观化重构。",
        "提取核心技术概念：多模态机械臂、自监督强化学习、零样本泛化，保留原作者 Two Minute Papers 署名。",
        "分区匹配：锁定 188 科技-计算机视觉/AI，标签规避低俗敏感词，保留硬核学术标签。",
        "根据策略规则『副号备份待人工审』，输出状态标记为待复审，等待操作者确认或微调。",
      ],
      candidateTitles: [
        "【中字】OpenAI 机械臂实现重大突破！自监督学习如何让机器人真正'学会思考'？",
        "【AI速递】5分钟看懂最新具身智能研究：自监督机械臂的零样本泛化能力",
        "Two Minute Papers前沿速递：多模态强化学习在真实机器人中的应用详解",
      ],
      selectedTitle:
        "【中字】OpenAI 机械臂实现重大突破！自监督学习如何让机器人真正'学会思考'？",
      description:
        "原作者: Two Minute Papers\n本视频由 Pi Agent 协同 Whisper 提取双语字幕并重构本地化元数据。\n\n【核心看点】\n1. 机械臂自监督预训练的收敛曲线\n2. 零样本重定向在不同物理抓取场景下的表现\n\n原视频链接: https://youtube.com/watch?v=dQw4w9WgXcQ",
      tags: ["人工智能", "机器人", "深度学习", "强化学习", "前沿科技", "OpenAI"],
      segments: [
        { part: 1, title: "P1: 实验背景与自监督训练架构", duration: "03:15" },
        { part: 2, title: "P2: 物理场景抓取实测与泛化总结", duration: "05:27" },
      ],
    },
    {
      id: "trace_002",
      sourceVideoId: "kJQP7kiw5Fk",
      sourceTitle: "Black Myth: Wukong - Official 4K Boss Fight Gameplay Breakdown",
      sourcePlatform: "youtube",
      sourceChannel: "IGN Official (游戏速递)",
      duration: "14:10",
      targetBiliAccount: "极客科技观察室 (主号)",
      biliMid: 18492011,
      whisperSummary:
        "IGN 实机演示黑神话悟空最新 Boss 战机制，解析动作连招、闪避判定窗口及虚幻5引擎的全局光照优化表现。",
      matchedRuleName: "科技主号全自动搬运",
      status: "auto_approved",
      createdAt: "1 小时前",
      llmModel: "pi-agent (deepseek-v3)",
      executionLatency: "1.8s",
      tokenUsage: 1210,
      reasoningChain: [
        "检测到高质量游戏实机演示，目标为主号全自动发布规则。",
        "自动优化标题结构：突出『4K 60帧』与『全机制拆解』以契合 B 站单机游戏区受众习惯。",
        "自动生成 B 站简介排版，注入时间戳导引，安全过滤检测通过，自动放行进入投稿队列。",
      ],
      candidateTitles: [
        "【4K60帧】黑神话悟空最新高能 Boss 战拆解！IGN 独家实机动作系统全解析",
        "【IGN中字】黑神话悟空白衣秀士高难战斗实录！虚幻引擎5顶级画面表现",
      ],
      selectedTitle:
        "【4K60帧】黑神话悟空最新高能 Boss 战拆解！IGN 独家实机动作系统全解析",
      description:
        "原视频来源: IGN\n画质: 4K 60FPS\n\nIGN 官方深度拆解黑神话悟空最新战斗系统！\n\n00:00 战斗开场与走位技巧\n04:20 第二阶段变身破招\n09:15 连招判定机制复盘",
      tags: ["黑神话悟空", "单机游戏", "游戏实况", "4K", "IGN", "虚幻5"],
      segments: [
        { part: 1, title: "P1: 4K Boss 战全程与招式拆解", duration: "14:10" },
      ],
      biliBvid: "BV1xx411c7Xz",
    },
    {
      id: "trace_003",
      sourceVideoId: "9bZkp7q19f0",
      sourceTitle: "Deleted Stream Archive: Twitch Live Policy Test",
      sourcePlatform: "twitch",
      sourceChannel: "shroud (Twitch 高能直播)",
      duration: "45:00",
      whisperSummary:
        "直播录制分段解析中检测到大段版权 BGM 背景音与源端流断流异常。",
      targetBiliAccount: "学术搬运存档库 (副号)",
      biliMid: 49201992,
      matchedRuleName: "副号备份待人工审",
      status: "deadletter",
      createdAt: "3 小时前",
      llmModel: "pi-agent (gpt-4o)",
      executionLatency: "1.2s",
      tokenUsage: 890,
      reasoningChain: [
        "音视频提取阶段检测到严重版权违规音频指纹匹配。",
        "媒体分段持续时间不足且 PTS 时间戳跳变，不满足 B 站投稿封装硬性指标。",
        "触发投稿防线终态熔断：判定为不可重试故障，隔离入死信队列待人工处置。",
      ],
      candidateTitles: ["【死信隔离】Shroud 直播切片存档 (异常)"],
      selectedTitle: "【死信隔离】Shroud 直播切片存档 (异常)",
      description: "该任务已触发死信保护，原因为版权黑名单拦截及流媒体时间戳断裂。",
      tags: ["死信", "异常归档"],
      segments: [],
      deadLetterReason: "版权音频指纹命中平台黑名单，且视频分段 PTS 异常 (Terminal Failure)",
    },
  ]);

  const [selectedId, setSelectedId] = useState<string>("trace_001");

  // 当前选中的决策追踪
  const activeTrace = traces.find((t) => t.id === selectedId) || traces[0];

  // 可编辑元数据状态
  const [editTitle, setEditTitle] = useState<string>(activeTrace?.selectedTitle || "");
  const [editDesc, setEditDesc] = useState<string>(activeTrace?.description || "");
  const [editTags, setEditTags] = useState<string[]>(activeTrace?.tags || []);
  const [newTagInput, setNewTagInput] = useState("");
  const [actionNotice, setActionNotice] = useState<string | null>(null);

  // 切换选中项时同步编辑框
  const handleSelectTrace = (item: DecisionTraceItem) => {
    setSelectedId(item.id);
    setEditTitle(item.selectedTitle);
    setEditDesc(item.description);
    setEditTags(item.tags);
    setActionNotice(null);
  };

  // 添加标签
  const handleAddTag = () => {
    const trimmed = newTagInput.trim();
    if (trimmed && !editTags.includes(trimmed) && editTags.length < 12) {
      setEditTags([...editTags, trimmed]);
      setNewTagInput("");
    }
  };

  // 删除标签
  const handleRemoveTag = (tagToRemove: string) => {
    setEditTags(editTags.filter((t) => t !== tagToRemove));
  };

  // 快捷应用候选标题
  const handleApplyCandidateTitle = (title: string) => {
    setEditTitle(title);
  };

  // 批准并通过投稿
  const handleApproveAndPublish = () => {
    setTraces((prev) =>
      prev.map((t) =>
        t.id === activeTrace.id
          ? {
              ...t,
              status: "auto_approved",
              selectedTitle: editTitle,
              description: editDesc,
              tags: editTags,
            }
          : t
      )
    );
    setActionNotice("已人工批准通过！任务已提交至 biliup 投稿工作流。");
  };

  // 驳回并隔离至死信
  const handleRejectToDeadletter = () => {
    setTraces((prev) =>
      prev.map((t) =>
        t.id === activeTrace.id
          ? {
              ...t,
              status: "deadletter",
              deadLetterReason: "操作者在人工审核台驳回：元数据或视频内容不合规",
            }
          : t
      )
    );
    setActionNotice("已驳回！该任务已转入死信队列，不会自动重试。");
  };

  // 过滤列表
  const filteredTraces = traces.filter((item) => {
    if (filterStatus !== "all" && item.status !== filterStatus) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      return (
        item.sourceTitle.toLowerCase().includes(q) ||
        item.selectedTitle.toLowerCase().includes(q) ||
        item.sourceVideoId.toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <AppLayout>
      <div className="space-y-6">
        {/* --- 顶部页头与统计条 --- */}
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
            <div className="flex items-center space-x-3">
              <div className="p-2.5 bg-purple-50 text-purple-600 rounded-lg">
                <Bot className="w-6 h-6" />
              </div>
              <div>
                <h1 className="text-xl font-bold text-gray-900">
                  Agent 决策追溯与人工审核台
                </h1>
                <p className="text-sm text-gray-500 mt-0.5">
                  全链路审查 Pi Agent 生成的标题、简介、标签与分 P 方案；支持一键放行、在线微调与死信隔离
                </p>
              </div>
            </div>

            {/* 状态过滤胶囊 */}
            <div className="flex items-center space-x-2 text-xs">
              <button
                onClick={() => setFilterStatus("all")}
                className={`px-3 py-1.5 rounded-full font-medium transition ${
                  filterStatus === "all"
                    ? "bg-gray-900 text-white"
                    : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                }`}
              >
                全部 ({traces.length})
              </button>
              <button
                onClick={() => setFilterStatus("pending_review")}
                className={`px-3 py-1.5 rounded-full font-medium transition ${
                  filterStatus === "pending_review"
                    ? "bg-amber-600 text-white"
                    : "bg-amber-50 text-amber-700 hover:bg-amber-100"
                }`}
              >
                待审核 ({traces.filter((t) => t.status === "pending_review").length})
              </button>
              <button
                onClick={() => setFilterStatus("auto_approved")}
                className={`px-3 py-1.5 rounded-full font-medium transition ${
                  filterStatus === "auto_approved"
                    ? "bg-blue-600 text-white"
                    : "bg-blue-50 text-blue-700 hover:bg-blue-100"
                }`}
              >
                已放行 ({traces.filter((t) => t.status === "auto_approved").length})
              </button>
              <button
                onClick={() => setFilterStatus("deadletter")}
                className={`px-3 py-1.5 rounded-full font-medium transition ${
                  filterStatus === "deadletter"
                    ? "bg-red-600 text-white"
                    : "bg-red-50 text-red-700 hover:bg-red-100"
                }`}
              >
                死信隔离 ({traces.filter((t) => t.status === "deadletter").length})
              </button>
            </div>
          </div>
        </div>

        {/* --- 主体分栏：左侧列表 + 右侧详情编辑 --- */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
          {/* 左侧：任务追溯列表 (4 列) */}
          <div className="lg:col-span-5 space-y-3">
            {/* 搜索栏 */}
            <div className="relative">
              <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="搜索原视频标题、BV号或ID..."
                className="w-full bg-white border border-gray-300 rounded-lg pl-9 pr-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
            </div>

            <div className="space-y-2">
              {filteredTraces.map((item) => {
                const isSelected = item.id === activeTrace?.id;
                return (
                  <div
                    key={item.id}
                    onClick={() => handleSelectTrace(item)}
                    className={`p-4 rounded-lg border cursor-pointer transition ${
                      isSelected
                        ? "bg-purple-50/50 border-purple-300 shadow-sm"
                        : "bg-white border-gray-200 hover:bg-gray-50"
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center space-x-1.5 text-xs text-gray-500">
                        <Video className="w-3.5 h-3.5 text-red-600" />
                        <span className="font-medium text-gray-700">
                          {item.sourceChannel}
                        </span>
                        <span>•</span>
                        <span>{item.duration}</span>
                      </div>

                      {/* 状态标签 */}
                      {item.status === "pending_review" && (
                        <span className="text-[11px] bg-amber-100 text-amber-800 px-2 py-0.5 rounded-full font-medium flex items-center">
                          <Clock className="w-3 h-3 mr-1" />
                          待人工审
                        </span>
                      )}
                      {item.status === "auto_approved" && (
                        <span className="text-[11px] bg-blue-100 text-blue-800 px-2 py-0.5 rounded-full font-medium flex items-center">
                          <CheckCircle2 className="w-3 h-3 mr-1" />
                          已放行
                        </span>
                      )}
                      {item.status === "deadletter" && (
                        <span className="text-[11px] bg-red-100 text-red-800 px-2 py-0.5 rounded-full font-medium flex items-center">
                          <AlertTriangle className="w-3 h-3 mr-1" />
                          死信隔离
                        </span>
                      )}
                    </div>

                    <h3 className="font-semibold text-gray-900 text-sm mt-2 line-clamp-2">
                      {item.selectedTitle}
                    </h3>

                    <div className="mt-2 text-xs text-gray-400 flex items-center justify-between">
                      <span className="font-mono">ID: {item.sourceVideoId}</span>
                      <span>{item.createdAt}</span>
                    </div>
                  </div>
                );
              })}

              {filteredTraces.length === 0 && (
                <div className="p-8 text-center text-gray-400 bg-white rounded-lg border border-dashed border-gray-300 text-sm">
                  暂无匹配的决策记录
                </div>
              )}
            </div>
          </div>

          {/* 右侧：决策依据推演与元数据在线微调 (7 列) */}
          <div className="lg:col-span-7 space-y-5">
            {activeTrace ? (
              <>
                {/* 动作反馈通知条 */}
                {actionNotice && (
                  <div className="p-3 bg-green-50 border border-green-200 text-green-800 text-sm rounded-lg flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      <Check className="w-4 h-4 text-green-600" />
                      <span>{actionNotice}</span>
                    </div>
                    <button
                      onClick={() => setActionNotice(null)}
                      className="text-green-600 hover:text-green-800"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </div>
                )}

                {/* 1. 顶部卡片：源视频与模型指标 */}
                <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5 space-y-4">
                  <div className="flex items-start justify-between border-b pb-3">
                    <div>
                      <div className="text-xs text-gray-400 uppercase tracking-wider font-semibold">
                        原生视频来源 (Source Context)
                      </div>
                      <div className="font-bold text-gray-900 text-base mt-1">
                        {activeTrace.sourceTitle}
                      </div>
                      <div className="flex items-center space-x-3 text-xs text-gray-500 mt-1">
                        <span>频道: {activeTrace.sourceChannel}</span>
                        <span>•</span>
                        <span>时长: {activeTrace.duration}</span>
                        <span>•</span>
                        <span className="font-mono">ID: {activeTrace.sourceVideoId}</span>
                      </div>
                    </div>

                    <div className="text-right flex-shrink-0">
                      <span className="text-xs bg-purple-50 text-purple-700 px-2 py-1 rounded font-mono font-medium block">
                        {activeTrace.llmModel}
                      </span>
                      <span className="text-[11px] text-gray-400 mt-1 block font-mono">
                        {activeTrace.executionLatency} / {activeTrace.tokenUsage} tokens
                      </span>
                    </div>
                  </div>

                  {/* Whisper 摘要提要 */}
                  <div>
                    <div className="text-xs font-semibold text-gray-600 mb-1 flex items-center space-x-1.5">
                      <FileText className="w-3.5 h-3.5 text-blue-600" />
                      <span>Whisper 语音识别提取提要 (Prompt Context)</span>
                    </div>
                    <p className="text-xs text-gray-600 bg-gray-50 p-2.5 rounded border border-gray-200/60 leading-relaxed">
                      {activeTrace.whisperSummary}
                    </p>
                  </div>
                </div>

                {/* 2. Agent 思考链追溯 (Reasoning Trace) */}
                <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5 space-y-3">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-bold text-gray-900 flex items-center space-x-2">
                      <Sparkles className="w-4 h-4 text-purple-600" />
                      <span>Agent 推理决策依据 (Reasoning Chain)</span>
                    </h3>
                    <span className="text-xs text-gray-400">
                      匹配规则: <strong>{activeTrace.matchedRuleName}</strong>
                    </span>
                  </div>

                  <div className="bg-slate-50 border border-slate-200 rounded-lg p-3 space-y-2">
                    {activeTrace.reasoningChain.map((step, idx) => (
                      <div key={idx} className="flex items-start space-x-2 text-xs">
                        <span className="w-4 h-4 bg-purple-100 text-purple-700 rounded-full flex items-center justify-center font-bold flex-shrink-0 text-[10px] mt-0.5">
                          {idx + 1}
                        </span>
                        <span className="text-slate-700 leading-relaxed">{step}</span>
                      </div>
                    ))}
                  </div>

                  {/* 死信原因告警 */}
                  {activeTrace.status === "deadletter" && activeTrace.deadLetterReason && (
                    <div className="p-3 bg-red-50 border border-red-200 rounded text-xs text-red-700 flex items-start space-x-2">
                      <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                      <div>
                        <strong>死信隔离原因：</strong>
                        {activeTrace.deadLetterReason}
                      </div>
                    </div>
                  )}
                </div>

                {/* 3. 候选标题对比与微调 (Candidate Titles) */}
                <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5 space-y-4">
                  <div>
                    <div className="flex items-center justify-between mb-1.5">
                      <label className="text-sm font-bold text-gray-900">
                        B 站投稿最终标题（不超过 80 字）
                      </label>
                      <span
                        className={`text-xs font-mono font-medium ${
                          editTitle.length > 80 ? "text-red-600 font-bold" : "text-gray-500"
                        }`}
                      >
                        {editTitle.length} / 80 字
                      </span>
                    </div>
                    <input
                      type="text"
                      value={editTitle}
                      onChange={(e) => setEditTitle(e.target.value)}
                      className={`w-full border rounded-lg p-2.5 text-sm font-medium focus:outline-none focus:ring-2 ${
                        editTitle.length > 80
                          ? "border-red-500 focus:ring-red-400"
                          : "border-gray-300 focus:ring-blue-500"
                      }`}
                    />
                  </div>

                  {/* 候选标题快捷切换列表 */}
                  <div>
                    <div className="text-xs font-semibold text-gray-500 mb-2">
                      Agent 生成的备选标题方案 (点击直接替换)：
                    </div>
                    <div className="space-y-1.5">
                      {activeTrace.candidateTitles.map((cand, idx) => (
                        <div
                          key={idx}
                          onClick={() => handleApplyCandidateTitle(cand)}
                          className={`p-2 rounded border text-xs cursor-pointer transition flex items-center justify-between ${
                            editTitle === cand
                              ? "bg-blue-50 border-blue-300 text-blue-900 font-medium"
                              : "bg-gray-50 border-gray-200 text-gray-700 hover:bg-gray-100"
                          }`}
                        >
                          <span className="line-clamp-1">{cand}</span>
                          <span className="text-gray-400 text-[10px] ml-2 flex-shrink-0 font-mono">
                            {cand.length}字
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* 简介编辑 */}
                  <div>
                    <label className="block text-sm font-bold text-gray-900 mb-1.5">
                      B 站动态简介内容
                    </label>
                    <textarea
                      rows={5}
                      value={editDesc}
                      onChange={(e) => setEditDesc(e.target.value)}
                      className="w-full border border-gray-300 rounded-lg p-2.5 text-xs text-gray-800 font-sans focus:outline-none focus:ring-2 focus:ring-blue-500 leading-relaxed"
                    />
                  </div>

                  {/* 标签编辑器 */}
                  <div>
                    <label className="block text-sm font-bold text-gray-900 mb-1.5">
                      视频标签 (最多 12 个)
                    </label>
                    <div className="flex flex-wrap gap-2 mb-2">
                      {editTags.map((tag) => (
                        <span
                          key={tag}
                          className="inline-flex items-center space-x-1 px-2.5 py-1 bg-purple-50 text-purple-700 rounded-full text-xs font-medium border border-purple-200"
                        >
                          <span>#{tag}</span>
                          <button
                            onClick={() => handleRemoveTag(tag)}
                            className="hover:text-purple-900 ml-1"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </span>
                      ))}
                    </div>

                    <div className="flex items-center space-x-2">
                      <input
                        type="text"
                        value={newTagInput}
                        onChange={(e) => setNewTagInput(e.target.value)}
                        onKeyDown={(e) => e.key === "Enter" && handleAddTag()}
                        placeholder="输入新标签按回车添加..."
                        className="border border-gray-300 rounded-lg px-3 py-1.5 text-xs flex-1"
                      />
                      <button
                        onClick={handleAddTag}
                        className="px-3 py-1.5 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg text-xs font-medium"
                      >
                        添加
                      </button>
                    </div>
                  </div>

                  {/* 多 P 分段命名展示 */}
                  {activeTrace.segments.length > 0 && (
                    <div>
                      <div className="text-xs font-bold text-gray-900 mb-1.5 flex items-center space-x-1.5">
                        <Layers className="w-3.5 h-3.5 text-blue-600" />
                        <span>多 P 稿件分段规划 ({activeTrace.segments.length}P)</span>
                      </div>
                      <div className="divide-y border rounded-lg bg-gray-50 text-xs">
                        {activeTrace.segments.map((seg) => (
                          <div
                            key={seg.part}
                            className="p-2.5 flex items-center justify-between"
                          >
                            <span className="font-medium text-gray-800">
                              {seg.title}
                            </span>
                            <span className="text-gray-400 font-mono">
                              {seg.duration}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* 4. 底部人工审核动作栏 */}
                  <div className="pt-4 border-t flex flex-col sm:flex-row items-center justify-between gap-3">
                    <div className="flex items-center space-x-2 w-full sm:w-auto">
                      <button
                        onClick={handleRejectToDeadletter}
                        className="w-full sm:w-auto px-4 py-2 border border-red-300 text-red-700 rounded-lg text-xs font-medium hover:bg-red-50 transition"
                      >
                        驳回至死信 (Reject)
                      </button>
                      <button
                        onClick={() => {
                          setActionNotice("已触发重新调用 Agent 生成！");
                        }}
                        className="w-full sm:w-auto px-3 py-2 border border-gray-300 text-gray-700 rounded-lg text-xs font-medium hover:bg-gray-50 flex items-center justify-center space-x-1"
                      >
                        <RotateCcw className="w-3.5 h-3.5" />
                        <span>重调决策</span>
                      </button>
                    </div>

                    <div className="flex items-center space-x-2 w-full sm:w-auto">
                      <button
                        onClick={handleApproveAndPublish}
                        disabled={editTitle.length > 80}
                        className="w-full sm:w-auto px-5 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-xs font-bold shadow transition flex items-center justify-center space-x-1.5 disabled:opacity-50"
                      >
                        <Send className="w-3.5 h-3.5" />
                        <span>人工批准并通过投稿</span>
                      </button>
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <div className="bg-white rounded-lg p-12 text-center text-gray-400">
                请在左侧选择要查看或审核的决策记录
              </div>
            )}
          </div>
        </div>
      </div>
    </AppLayout>
  );
}
