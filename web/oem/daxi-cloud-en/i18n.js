(function () {
  const storageKey = 'daxi-cloud-language';

  const translations = {
    'DAXI Technology': '大汐科技',
    'AI Model Gateway': 'AI 模型网关',
    'ASEAN AI Platform': '东盟 AI 平台',
    'Solutions': '行业方案',
    'Platform': '平台能力',
    'OEM': 'OEM 接入',
    'Pricing': '套餐计费',
    'Operations Hub': '运营中枢',
    'Contact': '联系支持',
    'Home': '首页',
    'Model API': '模型 API',
    'AI Compute': 'AI 算力',
    'Agents': 'Agent',
    'Short Drama': '短剧生成',
    'E-commerce Video': '电商短视频',
    'Docs': '文档',
    'Plans': '套餐',
    'Resources': '资源',
    'Get Started': '立即接入',
    'Menu': '菜单',
    'Model API Access': '模型 API 接入',
    'Book a Demo': '预约演示',
    'View Solutions': '查看方案',
    'Learn More': '了解更多',
    'Start Creating': '开始创作',
    'View API Docs': '查看 API 文档',
    'Choose Token Plan': '选择套餐',
    'View Quickstart': '查看接入流程',
    'Request Access': '申请接入',
    'Request Proposal': '申请方案',
    'Talk to Sales': '联系销售',
    'Submit Request': '提交需求',
    'Select expected volume': '请选择使用量',
    'API testing': 'API 测试',
    'Campaign production': '内容生产',
    'Enterprise workload': '企业工作负载',
    'GPU / private deployment': 'GPU / 私有化部署',
    'Choose budget range': '请选择预算范围',
    'Trial / PoC': '试用 / PoC',
    'Monthly subscription': '月度订阅',
    'Custom compute quote': '定制算力报价',

    'DAXI CLOUD OEM PLATFORM': 'DAXI CLOUD OEM 平台',
    'Launch Your': '上线你的',
    'Branded AI Cloud Platform': '品牌化 AI 云平台',
    'Model APIs, token billing, reseller management, and industry AI solutions in one OEM-ready portal.': '模型 API、Token 计费、经销商客户管理和行业 AI 应用，集中在一个可 OEM 交付的门户里。',
    '40+ Model Endpoints': '40+ 模型端点',
    'Token Backflow Control': 'Token 回流控制',
    'OEM Ready': '可 OEM 交付',
    'DAXI CLOUD': 'DAXI CLOUD',
    'AI Cloud Core': 'AI Cloud Core',
    'Routing • Metering • Settlement': 'Routing • Metering • Settlement',
    'Overview': '总览',
    'Model Gateway': '模型网关',
    'model endpoints': 'model endpoints',
    'Playground': '调试台',
    'Token & Billing': 'Token 与计费',
    'Reseller Management': '客户管理',
    'Token Ledger': 'Token 账本',
    'tracked calls': 'tracked calls',
    'Reseller Console': '经销商控制台',
    'active partners': 'active partners',
    'Usage Meter': '用量计量',
    'uptime': 'uptime',
    'API Request': 'API Request',
    'Validated': 'Validated',
    'Token Backflow': 'Token Backflow',
    'Settled': 'Settled',
    'Avg. Latency': 'Avg. Latency',
    'API Usage': 'API Usage',
    'Analytics': '数据分析',
    'API Keys': 'API Keys',
    'Live Status • All Systems Operational': '实时状态 • 系统运行正常',
    'Total API Calls': 'API 调用量',
    'Active Tokens': '活跃 Token',
    'Avg. Latency': '平均延迟',
    'Uptime': '可用性',
    'Smart routing': '智能路由',
    '7 days': '近 7 天',
    'API Calls': 'API 调用趋势',
    'Top Models': '热门模型',
    'Recent API Usage': '最近调用',
    'Partner A': '客户 A',
    'Partner B': '客户 B',
    'Partner C': '客户 C',
    'Success': '成功',
    'Unified Model Gateway': '统一模型网关',
    'Access 40+ model endpoints through one routed API gateway with failover.': '通过一个路由 API 网关接入 40+ 模型端点，并支持备用切换。',
    'Unified access to 40+ leading AI models with intelligent routing, failover, and usage optimization.': '统一接入 40+ 主流 AI 模型，支持智能路由、备用切换和用量优化。',
    'Access 40+ model endpoints through a single API gateway with routing and failover.': '通过一个 API 入口接入多模型能力，支持场景路由、备用模型与调用控制。',
    'Track usage, post recharges, review orders, and keep token backflow auditable.': '追踪用量、记录充值、复核订单，并保持 Token 回流可审计。',
    'Flexible token accounting, usage tracking, manual recharge, and detailed reports.': '支持预付 Token、用量记录、人工充值、订单和支付凭证管理。',
    'Flexible token accounting, auto recharge, usage tracking, and custom pricing with detailed reports.': '支持灵活 Token 账本、自动充值、用量追踪、自定义定价和详细报表。',
    'Create customer accounts, issue downstream keys, and monitor usage in real time.': '创建客户账号、签发下游 Key，并实时监控用量。',
    'Manage customer accounts, issue downstream keys, and monitor usage in real time.': '创建客户账号、签发下游 Key，并实时监控用量。',
    'Manage customers, allocate budgets, issue API keys, and monitor usage in real time.': '统一管理客户、API Key、余额、模型权限和实时用量。',
    'Manage sub-resellers, allocate budgets, monitor usage, and grow your AI business with ease.': '管理下级经销商、分配预算、监控用量，并轻松扩展 AI 业务。',
    'Industry Solutions': '行业应用方案',
    'Launch ready-to-use AI workflows for content, commerce, support, and knowledge.': '上线面向内容、电商、客服和知识场景的 AI 工作流。',
    'Ready-to-use AI workflows for content, commerce, customer service, and knowledge.': '面向内容、电商、客服和知识场景提供可落地的 AI 工作流。',
    'Ready-to-use AI solutions for content, commerce, service, and enterprise applications.': '面向内容、电商、服务和企业应用提供开箱即用的 AI 方案。',
    'INDUSTRY SOLUTIONS': '行业解决方案',
    'INDUSTRY SOLUTION': '行业解决方案',
    'AI Solutions Built for Real Business Impact': '为真实业务增长设计的 AI 应用',
    'AI Solutions Built for Business Growth': '为业务增长打造的 AI 解决方案',
    'Power real business scenarios with DAXI Cloud AI solutions.': '用 DAXI Cloud AI 方案驱动真实业务场景。',
    'Short Drama Generation': '短剧生成',
    'Generate engaging short dramas from scripts with AI-powered storyboard, scenes, and character visuals.': '从剧情创意到脚本、分镜、角色视觉和竖屏视频方案，快速生成短剧内容。',
    'E-commerce Short Video': '电商短视频生成',
    'Create product highlight videos and marketing clips to boost engagement and drive conversions.': '根据商品卖点生成脚本、画面、字幕和多版本营销短视频。',
    'Customer Support AI': '智能客服 AI',
    'AI agents that understand, respond, and resolve customer inquiries fast and accurately.': '帮助企业搭建可理解、可回复、可沉淀知识的客服助手。',
    'AI agents that understand, respond, and resolve customer inquiries fast and accurately across channels.': '通过多渠道智能体快速、准确地理解、回复并解决客户咨询。',
    'Knowledge Assistant': '知识库助手',
    'Build AI assistants that search, understand, and summarize company knowledge in seconds.': '对企业资料进行检索、理解、总结和问答，提升团队效率。',
    'Build AI assistants that search, understand, and summarize your company knowledge in seconds.': '搭建可检索、理解并快速总结企业知识的 AI 助手。',
    'Platform Uptime': '平台可用性',
    'Model Endpoints': '模型端点',
    'Reseller Partners': '伙伴客户',
    'Secure Infrastructure': '安全基础设施',
    'Global': '全球',
    'System Health': 'System Health',
    'API Gateway': 'API Gateway',
    'Billing Service': 'Billing Service',
    'Model Routing': 'Model Routing',
    'Healthy': 'Healthy',
    'Balance Tokens': 'Balance Tokens',
    '40+ Models Available': '40+ Models Available',
    'Your App': 'Your App',
    'Web': 'Web',
    'Mobile': 'Mobile',
    'View all': 'View all',

    'Start Your OEM Integration': '开始你的 OEM 平台接入',
    'Connect your domain, configure model access, create reseller packages, and launch your branded AI service portal.': '绑定客户域名、配置模型访问、创建 Token 套餐，并上线带客户品牌的 AI 服务门户。',
    'Selected scenario:': '已选场景：',
    'Brand Setup': '品牌配置',
    'Configure your brand and platform identity.': '配置 Logo、域名和平台身份。',
    'API & Model Access': '模型接入',
    'Select models and set access levels.': '选择模型并设置访问范围。',
    'Token Billing': 'Token 计费',
    'Configure pricing, packages, and limits.': '配置套餐、价格和额度。',
    'Launch Review': '上线复核',
    'Review settings and launch your portal.': '确认配置后发布平台。',
    'Integration Progress': '接入进度',
    'Completed': '已完成',
    'Logo, domain, and platform details': 'Logo、域名和平台信息',
    'Done': '已完成',
    'Select models and configure access': '选择模型并设置权限',
    'In Progress': '进行中',
    'Create packages and billing rules': '创建套餐和计费规则',
    'Pending': '待处理',
    'Review and launch the platform': '检查并发布平台',
    'Launch review': '上线复核',
    'OEM Launch Checklist': '上线检查清单',
    'Company information': '企业信息',
    'Domain configuration': '域名配置',
    'Brand assets upload': '品牌素材上传',
    'Model access setup': '模型访问设置',
    'Billing & packages': '计费与套餐',
    'Reseller roles': '经销商角色',
    'Platform Overview': '平台总览',
    'API Calls (30 days)': 'API 调用量（30 天）',
    'Uptime (30 days)': '可用性（30 天）',
    'Integration Request': '接入需求提交',
    'Tell us about your business and integration needs. Our team will contact you within 1 business day.': '填写业务与接入需求，我们将在 1 个工作日内联系你。',
    'Company Name': '公司名称',
    'Business Email': '业务邮箱',
    'Target Market': '目标市场',
    'Expected Monthly Volume': '预计月使用量',
    'Budget Range': '预算范围',
    'Additional Information': '补充说明',
    '1-business-day response': '1 个工作日响应',
    'Secure & confidential': '信息安全保密',
    'White-label Portal': '白标门户',
    'Fully branded portal for your models and services.': '为模型和服务提供完整品牌化门户。',
    'Global Infrastructure': '全球基础设施',
    'High-performance, reliable infrastructure worldwide.': '覆盖全球的高性能可靠基础设施。',
    'Flexible Billing': '灵活计费',
    'Token-based billing with custom packages and limits.': '基于 Token 的计费，可配置套餐和额度。',
    'Expert Support': '专家支持',
    'Dedicated OEM success team for your growth.': '专属 OEM 成功团队支持业务增长。',
    'Submit Compute Requirement': '提交算力需求',
    'View Resource Types': '查看资源类型',
    'Plan an Agent PoC': '规划 Agent PoC',
    'View Agent Packs': '查看 Agent 套餐',

    'MODEL API & TOKEN ACCESS': '模型 API 与 Token 接入',
    'One OpenAI-compatible API path for multiple model capabilities.': '一个兼容 OpenAI 的 API 入口，统一接入多模型能力。',
    'Give developers and business teams a clean API entrance, prepaid token packages, usage visibility, and scenario-based access control.': '面向开发者和业务团队提供清晰的 API 接入、预付 Token 套餐、用量可视化和场景权限控制。',
    'API QUICKSTART': 'API 接入流程',
    'From token purchase to first API call.': '从购买 Token 到完成首次调用。',
    'DAXI Cloud issues downstream customer keys, applies package limits, records local usage, and forwards final model calls through the controlled upstream gateway.': 'DAXI Cloud 为下游客户签发 API Key、校验套餐余额、记录本地用量，并通过受控上游网关完成最终模型调用。',
    '1. Select token package': '1. 选择 Token 套餐',
    'Choose trial, growth, or custom business volume.': '按试用、增长或定制业务量选择额度。',
    '2. Create customer API key': '2. 创建客户 API Key',
    'Set allowed models, scenarios, quota, and rate limit.': '设置模型范围、场景权限、额度和速率限制。',
    '3. Call OpenAI-compatible endpoint': '3. 调用兼容 OpenAI 的接口',
    'Use familiar SDKs and endpoint format to reduce integration effort.': '复用熟悉的 SDK 和 endpoint 格式，降低接入成本。',
    '4. Monitor usage and balance': '4. 监控用量和余额',
    'Track usage by customer, key, model, time, and scenario.': '按客户、Key、模型、时间和场景追踪 Token 消耗。',
    'TOKEN PACKAGES': 'Token 套餐',
    'Prepaid access for testing, campaigns, and business workloads.': '覆盖测试、营销活动和企业级工作负载。',
    'API Trial Pack': 'API 试用包',
    'For model testing, prototype integration, and first customer validation.': '适合模型测试、原型接入和首批客户验证。',
    'Business API Pack': '商业 API 套餐',
    'For agencies, application teams, commerce workflows, and recurring API usage.': '适合代理商、应用团队、电商工作流和持续 API 调用。',
    'Enterprise Volume': '企业定制额度',
    'Custom quota': '定制额度',
    'For private deployment, dedicated support, special routing, and settlement needs.': '适合私有化部署、专属支持、特殊路由和结算需求。',
    'OPERATION CONTROL': '运营控制',
    'DAXI customers use DAXI keys. Final token metering remains controlled upstream.': 'DAXI 客户使用 DAXI Key，最终 Token 计量仍由上游统一控制。',
    'DAXI Cloud can manage local packages and downstream customers, while final model supply, DAXI upstream quota, settlement, and emergency stop remain controlled by the master gateway.': 'DAXI Cloud 管理本地套餐和下游客户，最终模型供应、上游额度、结算和紧急停用仍由主网关控制。',
    'DAXI Customer Key': 'DAXI 客户 Key',
    'DAXI Package Check': 'DAXI 套餐校验',
    'DAXI Upstream Key': 'DAXI 上游 Key',
    'Final Token Metering': '最终 Token 计量',

    'Home › Solutions › Industry Solutions › Short Drama Generation': '首页 › 行业方案 › 行业解决方案 › 短剧生成',
    'Generate Short Drama Scripts, Scenes, and Video Assets': '生成短剧脚本、场景和视频资产',
    'with AI': '通过 AI',
    'Turn a plot idea into scripts, characters, storyboards, voice prompts, and publish-ready vertical video plans.': '将剧情创意快速转化为角色设定、分集脚本、镜头分镜、配音提示和竖屏发布方案。',
    'Contract Romance': '契约爱情',
    'Rain Encounter': '雨夜相遇',
    'Office Reveal': '办公室揭晓',
    'Rooftop Night': '天台夜景',
    'Short Video Ratio': '短视频比例',
    'Completion Rate': '完播率',
    'Engagement Rate': '互动率',
    'Avg. Rating': '平均评分',
    'Script Generator': '脚本生成',
    'Generate engaging scripts by genre, tone, and episode length.': '按题材、情绪、集数和人物关系生成完整短剧脚本。',
    'Character Builder': '角色构建',
    'Create detailed character profiles and relationships.': '生成角色画像、人物关系、造型描述和表演方向。',
    'Scene Storyboard': '分镜规划',
    'Visualize key scenes with AI-generated storyboards and shot lists.': '将脚本拆成镜头、场景、时长和画面描述。',
    'Voice & Subtitle Plan': '配音字幕',
    'Generate voice prompts, subtitles, and caption styles.': '生成配音提示、多语言字幕和短视频发布素材。',
    'AI-Powered Short Drama Creation Workflow': 'AI 短剧创作流程',
    'Idea & Plot': '创意输入',
    'Generate Script': '生成脚本',
    'Build Characters': '构建角色',
    'Storyboard Scenes': '分镜可视化',
    'Voice & Subtitles': '配音字幕',
    'Export & Publish': '导出发布',
    'High-Quality Scripts': '高质量脚本',
    'Engaging plots that keep audiences watching.': '生成更容易留住观众的剧情脚本。',
    'Cinematic Visuals': '电影感视觉',
    'AI-generated storyboards and scene guidance.': 'AI 生成分镜和场景画面指导。',
    'Multi-language Support': '多语言支持',
    'Voices and subtitles in multiple languages.': '支持多语言配音和字幕。',
    'Vertical-First Workflow': '竖屏优先流程',
    'Optimized for short video platforms.': '面向短视频平台优化。',
    'Chase Lin': '林澈',
    'Male Lead • CEO • Dominant': '男主 • CEO • 强势',
    'Su Yihan': '苏依涵',
    'Female Lead • Designer • Independent': '女主 • 设计师 • 独立',
    'You said this was just a contract.': '你说过这只是一纸合约。',

    'Home › Solutions › Industry Solutions › E-commerce Short Video Generation': '首页 › 行业方案 › 行业解决方案 › 电商短视频生成',
    'Create Product Short Videos for Every Storefront': '为每个商品生成高转化短视频',
    'Generate hooks, scripts, product scenes, captions, voiceovers, and vertical ads from product details.': '根据商品卖点自动生成开场钩子、脚本、画面提示、字幕、配音和多版本竖屏广告。',
    'Beauty Demo': '美妆展示',
    'Skincare serum highlight': '精华卖点展示',
    'Electronics Unboxing': '数码开箱',
    'Feature showcase': '功能亮点展示',
    'Fashion Try-On': '服饰试穿',
    'Outfit display': '穿搭效果展示',
    'Kitchen Gadget': '厨房小家电',
    'Usage demo': '使用场景演示',
    'Lifestyle Product': '生活方式商品',
    'Daily scenario': '日常场景',
    'Product Script': '商品脚本',
    'Automatically generate engaging hooks and scripts based on product highlights and benefits.': '基于商品信息、卖点和目标人群生成营销脚本。',
    'Scene Templates': '场景模板',
    'Pre-built vertical video templates optimized for e-commerce platforms and campaigns.': '内置适合电商平台和广告投放的竖屏模板。',
    'Auto Captions': '自动字幕',
    'Smart captions and subtitles increase engagement and improve watch time.': '生成字幕、标题和多语言文案，提高完播率。',
    'Multi-language Ads': '多语言广告',
    'Generate localized voiceovers and captions to reach global customers effortlessly.': '支持跨境市场的配音、字幕和本地化素材。',
    'Performance Overview': '效果总览',
    'Last 7 days': '近 7 天',
    'CTR Lift': 'CTR 提升',
    'vs. previous 7 days': '对比上一周期',
    'Video Variants': '视频版本',
    'Generated': '已生成',
    'Ad Spend Saved': '节省投放成本',
    'Estimated savings': '预估节省',
    'Why It Works': '为什么有效',
    'Built for e-commerce and short video platforms': '面向电商和短视频平台设计',
    'AI-generated content that converts': 'AI 生成更易转化的内容',
    'Fast production, lower cost, higher ROI': '生产更快、成本更低、ROI 更高',
    'Scale globally with multi-language support': '通过多语言支持扩展全球市场',
    'Learn more about our video generation API': '了解视频生成 API',
    'Designed for sellers, agencies, and cross-border commerce teams.': '为卖家、代理商和跨境电商团队设计。',
    'Move from SKU data to campaign-ready video variations with templates, scripts, captions, and export-ready assets.': '从 SKU 数据快速生成可投放的视频版本，包括模板、脚本、字幕和导出素材。',

    'Book a Demo': '预约演示',
    'Plans': '套餐计费',
    'Resources': '资源',
    'View API Reference': '查看 API 参考',
    'Starter': '入门',
    'Growth': '增长',
    'Custom': '定制',
    'Latency': '延迟',
    'Fallback': '备用切换',
    'Metering': '计量',
    'Token-level': 'Token 级',
    'Live': '实时',
    'Additional Information': '补充说明',
    'Enter your company name': '请输入公司名称',
    'Enter your business email': '请输入业务邮箱',
    'DAXI Technology branded AI cloud platform.': '大汐科技品牌化 AI 云平台。',
    '© 2026 DAXI Technology. All rights reserved.': '© 2026 大汐科技。保留所有权利。',

    'Website': '官网',
    'API Docs': 'API 文档',
    'Connecting to backend...': '正在连接后端...',
    'DAXI Admin': 'DAXI 管理',
    'Dashboard': '仪表盘',
    'Leads': '线索',
    'Manual Handoff': '人工交接',
    'Customer Ledger': '客户账本',
    'Usage': '用量',
    'Settings': '设置',
    'FIRST-PHASE OPERATIONS': '首期运营',
    'Lead intake and manual onboarding for DAXI Cloud resellers.': 'DAXI Cloud 经销商线索接收与人工开通。',
    'Public registration only creates customer leads in this console. API keys, token packages, and recharge operations are handled by platform operations offline.': '公开注册仅在本后台生成客户线索。API Key、Token 套餐和充值操作由平台运营线下处理。',
    'Admin sign in': '管理员登录',
    'Username': '用户名',
    'Password': '密码',
    'Sign In': '登录',
    'Advanced connection': '高级连接',
    'Admin token': '管理员令牌',
    'Connect': '连接',
    'New Leads': '新增线索',
    'Get Started submissions': '接入页提交',
    'Customers': '客户',
    'Platform-managed': '平台托管',
    'Issued by platform ops': '平台运营签发',
    'Total Tokens': 'Token 总量',
    'Tracked usage': '已追踪用量',
    'Proxy requests': '代理请求',
    'Upstream Status': '上游状态',
    'Main platform control': '主平台控制',
    'Video Tasks': '视频任务',
    'Scenario queue': '场景队列',
    'Control Mode': '控制模式',
    'Manual': '人工',
    'P0 delivery scope': 'P0 交付范围',
    'Lead received': '收到线索',
    'Customer submits Get Started form.': '客户提交接入表单。',
    'Ops qualifies': '运营筛选',
    'Review scenario, volume, and contact details.': '核对场景、用量和联系方式。',
    'Platform handoff': '平台交接',
    'Send qualified lead to platform operations.': '将合格线索交给平台运营。',
    'Manual activation': '人工开通',
    'Platform ops creates customer, key, and token package.': '平台运营创建客户、Key 和 Token 套餐。',
    'Track usage': '追踪用量',
    'Review downstream usage and settlement records.': '查看下游用量和结算记录。',
    'LEADS CRM': '线索 CRM',
    'Customer registration intake': '客户注册线索',
    'All public Get Started submissions land here for manual follow-up.': '所有公开接入申请都会进入这里，供运营人工跟进。',
    'All statuses': '全部状态',
    'New': '新增',
    'Contacted': '已联系',
    'Quoting': '报价中',
    'PoC': 'PoC',
    'Won': '已成交',
    'Closed': '已关闭',
    'Export CSV': '导出 CSV',
    'Load sample leads': '加载示例线索',
    'Select a lead to review details.': '选择一条线索查看详情。',
    'Contact information, scenario notes, and handoff guidance will appear here.': '联系方式、场景备注和交接建议会显示在这里。',
    'MANUAL HANDOFF': '人工交接',
    'Phase-one operating boundary': '首期运营边界',
    'DAXI can receive and qualify customer registrations here. Customer accounts, API keys, token recharge, and billing activation remain platform-operated until the Customer Console is approved.': 'DAXI 可在这里接收并筛选客户注册。客户账号、API Key、Token 充值和计费开通在客户控制台批准前仍由平台运营处理。',
    'Visible to DAXI': 'DAXI 可见',
    'Lead list, lead status, customer/contact details, usage overview, issued-key ledger.': '线索列表、线索状态、客户/联系人信息、用量概览和已签发 Key 账本。',
    'Handled by platform ops': '平台运营处理',
    'Customer creation, API key issuance, token package setup, recharge posting, and settlement checks.': '客户创建、API Key 签发、Token 套餐配置、充值入账和结算核对。',
    'Not included in phase one': '首期不包含',
    'Customer self-service login, automatic API key generation, self-recharge, and online payment.': '客户自助登录、自动生成 API Key、自助充值和在线支付。',
    'CUSTOMER LEDGER': '客户账本',
    'Platform-managed customers and issued keys': '平台托管客户与已签发 Key',
    'This is read-only for reseller operations. Creation and recharge are controlled by platform operations.': '此处对经销商运营只读。客户创建和充值由平台运营控制。',
    'CUSTOMER DETAIL': '客户详情',
    'Balance, keys, and usage summary': '余额、Key 和用量汇总',
    'BILLING RECORDS': '计费记录',
    'Orders and token recharge ledger': '订单与 Token 充值账本',
    'Read-only audit trail. Recharge actions are posted by platform operations.': '只读审计记录。充值动作由平台运营入账。',
    'USAGE LEDGER': '用量账本',
    'Recent model proxy records': '最近模型代理记录',
    'ADMIN ACCESS': '管理员权限',
    'Operations users': '运营用户',
    'Create internal operator accounts. Customer self-service console is intentionally out of scope for phase one.': '创建内部运营账号。客户自助控制台不在首期范围内。',
    'Role': '角色',
    'Operator': '运营',
    'Finance': '财务',
    'Owner': '所有者',
    'Create Admin': '创建管理员',
    'No submitted requirements yet.': '暂无提交的接入需求。',
    'Use Get Started to submit a requirement, or load sample leads for demo.': '可通过接入页提交需求，或加载示例线索演示。',
    'No leads match the current filters.': '没有符合当前筛选条件的线索。',
    'Clear the search keyword or choose another status.': '清空搜索关键词或选择其他状态。',
    'ID': 'ID',
    'Scenario': '场景',
    'Company': '公司',
    'Contact': '联系人',
    'Status': '状态',
    'Created': '创建时间',
    'Action': '操作',
    'Update': '更新',
    'Details': '详情',
    'Unqualified Lead': '待筛选线索',
    'Scenario pending': '场景待确认',
    'Country': '国家/地区',
    'Usage Profile': '使用画像',
    'Budget / Package': '预算 / 套餐',
    'Source': '来源',
    'public form': '公开表单',
    'Notes': '备注',
    'Recommended handoff': '建议交接',
    'Confirm contact and usage scope, then send the qualified lead to platform operations for customer record, API key, token package, and recharge setup.': '确认联系方式和使用范围后，将合格线索交给平台运营处理客户档案、API Key、Token 套餐和充值配置。',
    'Follow-up note': '跟进记录',
    'Next step': '下一步',
    'Add Follow-up': '添加跟进',
    'Activity unavailable.': '跟进记录暂不可用。',
    'No follow-up activity yet.': '暂无跟进记录。',
    'Add a note after the first customer contact.': '首次联系客户后添加一条记录。',
    'No customers yet.': '暂无客户。',
    'Create a customer to issue downstream API keys.': '创建客户后可签发下游 API Key。',
    'Customer': '客户',
    'Balance': '余额',
    'No API keys yet.': '暂无 API Key。',
    'Select a customer and issue the first downstream key.': '选择客户后签发第一个下游 Key。',
    'Key': 'Key',
    'Customer ID': '客户 ID',
    'Scope': '范围',
    'No token orders yet.': '暂无 Token 订单。',
    'Create an order from a customer and plan.': '根据客户和套餐创建订单。',
    'Order': '订单',
    'Plan': '套餐',
    'Tokens': 'Token',
    'No recharge records yet.': '暂无充值记录。',
    'Manual top-ups will appear here.': '人工充值记录会显示在这里。',
    'Recharge': '充值',
    'No usage records yet.': '暂无用量记录。',
    'Completed model proxy calls with usage will appear here.': '已完成的模型代理调用及用量会显示在这里。',
    'Request': '请求',
    'Model': '模型',
    'No model routes yet.': '暂无模型路由。',
    'Create scenario routes before opening downstream access.': '开放下游接入前请先创建场景路由。',
    'Primary model': '主模型',
    'Fallbacks': '备用模型',
    'Limit': '限制',
    'No video tasks yet.': '暂无视频任务。',
    'Short drama and e-commerce video jobs will appear here.': '短剧和电商短视频任务会显示在这里。',
    'Task': '任务',
    'Type': '类型',
    'Progress': '进度',
    'No payment records yet.': '暂无付款记录。',
    'Offline transfers, invoices, and payment confirmations will appear here.': '线下转账、发票和付款确认会显示在这里。',
    'Payment': '付款',
    'Amount': '金额',
    'Reference': '参考号',
    'No admin users yet.': '暂无管理员用户。',
    'The default owner account will appear after backend startup.': '后端启动后会显示默认所有者账号。',
    'User': '用户',
    'Last login': '最后登录',
    'Loading customer detail...': '正在加载客户详情...',
    'available tokens': '可用 Token',
    'issued keys': '已签发 Key',
    'Total Usage': '总用量',
    'requests': '次请求',
    'Connected to': '已连接到',
    'Please sign in to load live operations data.': '请先登录以加载实时运营数据。',
    'Signing in...': '正在登录...',
    'Signed in with admin session.': '已登录管理员会话。',
    'invalid admin credentials': '管理员账号或密码错误',
    'Creating admin user...': '正在创建管理员...',
    'Saving...': '正在保存...',
    'Saved': '已保存',
    'Saved local': '已保存到本地',
    'Failed': '失败',
    'Add a follow-up note or next step.': '请填写跟进记录或下一步。',
    'Sample leads do not persist activity.': '示例线索不会持久化跟进记录。',
    'Saving follow-up...': '正在保存跟进记录...',
    'Follow-up saved.': '跟进记录已保存。'
  };

  const titleTranslations = {
    'DAXI Cloud | Branded AI Cloud Platform': 'DAXI Cloud | 品牌化 AI 云平台',
    'Start OEM Integration | DAXI Cloud': '开始 OEM 接入 | DAXI Cloud',
    'Model API & Token Access | DAXI Cloud': '模型 API 与 Token 接入 | DAXI Cloud',
    'Short Drama Generation | DAXI Cloud': '短剧生成 | DAXI Cloud',
    'E-commerce Short Video Generation | DAXI Cloud': '电商短视频生成 | DAXI Cloud',
    'Operations Hub | DAXI Cloud': '运营中枢 | DAXI Cloud'
  };

  const placeholderTranslations = {
    'Enter your company name': '请输入公司名称',
    'Enter your business email': '请输入邮箱',
    'Malaysia, Singapore, Indonesia...': '马来西亚 / 新加坡 / 印尼...',
    'Tell us more about your business or integration requirements...': '请描述业务场景、模型需求或上线计划...',
    'daxi': 'daxi',
    'Temporary password': '临时密码',
    'Session token': '会话令牌',
    'Search company, contact, scenario...': '搜索公司、联系人、场景...',
    'daxi-ops': 'daxi-ops',
    'Record call summary, qualification result, or customer requirement...': '记录通话摘要、筛选结果或客户需求...',
    'Example: send pricing, schedule demo, wait for platform activation': '例如：发送报价、安排演示、等待平台开通'
  };

  const reverseTranslations = Object.fromEntries(Object.entries(translations).map(([en, zh]) => [zh, en]));
  const reversePlaceholders = Object.fromEntries(Object.entries(placeholderTranslations).map(([en, zh]) => [zh, en]));

  function translateText(value, dictionary) {
    const trimmed = value.trim();
    if (!trimmed) return value;
    const suffixArrow = trimmed.endsWith(' →') ? ' →' : trimmed.endsWith('→') ? ' →' : '';
    const base = suffixArrow ? trimmed.replace(/\s*→$/, '') : trimmed;
    const translated = dictionary[trimmed] || (suffixArrow && dictionary[base] ? dictionary[base] + suffixArrow : '');
    if (!translated) return value;
    return value.replace(trimmed, translated);
  }

  function rememberOriginals(node) {
    if (!node) return;
    if (node.nodeType === Node.TEXT_NODE) {
      const trimmed = node.nodeValue.trim();
      if (trimmed && !node.__daxiOriginalText) node.__daxiOriginalText = node.nodeValue;
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    if (['SCRIPT', 'STYLE', 'PRE', 'CODE'].includes(node.tagName)) return;
    if (node.hasAttribute('data-i18n-skip')) return;
    if (node.hasAttribute('placeholder') && !node.dataset.i18nPlaceholder) {
      node.dataset.i18nPlaceholder = node.getAttribute('placeholder') || '';
    }
    if (node.hasAttribute('alt') && !node.dataset.i18nAlt) {
      node.dataset.i18nAlt = node.getAttribute('alt') || '';
    }
    if (node.hasAttribute('aria-label') && !node.dataset.i18nAriaLabel) {
      node.dataset.i18nAriaLabel = node.getAttribute('aria-label') || '';
    }
    node.childNodes.forEach((child) => rememberOriginals(child));
  }

  function walk(node, dictionary, lang) {
    if (!node) return;
    if (node.nodeType === Node.TEXT_NODE) {
      const original = node.__daxiOriginalText;
      if (!original) return;
      node.nodeValue = lang === 'zh' ? translateText(original, dictionary) : original;
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    if (['SCRIPT', 'STYLE', 'PRE', 'CODE'].includes(node.tagName)) return;
    if (node.hasAttribute('data-i18n-skip')) return;
    node.childNodes.forEach((child) => walk(child, dictionary, lang));
  }

  function translateAttributes(dictionary, lang) {
    document.querySelectorAll('[placeholder]').forEach((item) => {
      const original = item.dataset.i18nPlaceholder || item.placeholder;
      item.placeholder = lang === 'zh' ? dictionary[original] || original : original;
    });
    document.querySelectorAll('[alt]').forEach((item) => {
      const original = item.dataset.i18nAlt || item.alt;
      item.alt = lang === 'zh' ? dictionary[original] || original : original;
    });
    document.querySelectorAll('[aria-label]').forEach((item) => {
      const original = item.dataset.i18nAriaLabel || item.getAttribute('aria-label');
      item.setAttribute('aria-label', lang === 'zh' ? dictionary[original] || original : original);
    });
  }

  function applyLanguage(lang) {
    rememberOriginals(document.body);
    const dictionary = lang === 'zh' ? translations : reverseTranslations;
    const placeholderDictionary = lang === 'zh' ? placeholderTranslations : reversePlaceholders;
    walk(document.body, dictionary, lang);
    translateAttributes(placeholderDictionary, lang);
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
    if (!document.documentElement.dataset.originalTitle) {
      document.documentElement.dataset.originalTitle = document.title;
    }
    const originalTitle = document.documentElement.dataset.originalTitle;
    document.title = lang === 'zh' ? titleTranslations[originalTitle] || originalTitle : originalTitle;
    document.body.classList.toggle('is-zh', lang === 'zh');
    document.querySelectorAll('.lang-switch a').forEach((item) => {
      const isActive = item.dataset.lang === lang || item.textContent.trim().toLowerCase() === lang;
      item.classList.toggle('active', isActive);
    });
    localStorage.setItem(storageKey, lang);
    localStorage.setItem('daxi-lang', lang);
  }

  function setupLanguageSwitch() {
    rememberOriginals(document.body);
    let observing = false;
    const observer = new MutationObserver((mutations) => {
      if (observing) return;
      const hasRelevantMutation = mutations.some((mutation) => mutation.type === 'childList' || mutation.type === 'characterData');
      if (!hasRelevantMutation) return;
      const lang = localStorage.getItem(storageKey);
      if (lang !== 'zh') return;
      observing = true;
      requestAnimationFrame(() => {
        applyLanguage('zh');
        observing = false;
      });
    });
    observer.observe(document.body, { childList: true, characterData: true, subtree: true });
    document.querySelectorAll('.lang-switch a').forEach((item) => {
      const label = item.textContent.trim().toLowerCase();
      item.dataset.lang = label === '中文' || label === 'zh' ? 'zh' : 'en';
      item.setAttribute('href', '#');
      item.addEventListener('click', (event) => {
        event.preventDefault();
        applyLanguage(item.dataset.lang);
      });
    });
    const urlLang = new URLSearchParams(window.location.search).get('lang');
    const preferred = urlLang === 'zh' || urlLang === 'en' ? urlLang : (localStorage.getItem(storageKey) || localStorage.getItem('daxi-lang'));
    if (preferred === 'zh' || preferred === 'en') applyLanguage(preferred);
  }

  window.DAXIApplyLanguage = applyLanguage;

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', setupLanguageSwitch);
  } else {
    setupLanguageSwitch();
  }
})();
