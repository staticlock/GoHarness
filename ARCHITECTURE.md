# GoHarness 架构设计图

## 📋 总体架构图

```mermaid
graph TB
    subgraph "用户界面层 (User Interface Layer)"
        A[React终端UI] --> B[CLI命令行]
        A --> C[打印模式]
        A --> D[Web界面]
    end
    
    subgraph "应用层 (Application Layer)"
        B --> E[CLI处理器]
        C --> E
        A --> F[UI控制器]
        D --> F
        E --> G[应用协调器]
        F --> G
    end
    
    subgraph "核心服务层 (Core Services Layer)"
        G --> H[AI服务]
        G --> I[工具服务]
        G --> J[会话服务]
        G --> K[插件服务]
        G --> L[权限服务]
        G --> M[状态服务]
    end
    
    subgraph "AI引擎层 (AI Engine Layer)"
        H --> N[查询引擎]
        N --> O[AI API客户端]
        O --> P[Anthropic Claude]
        O --> Q[OpenAI GPT]
        O --> R[其他模型]
        N --> S[记忆管理]
        N --> T[上下文管理]
    end
    
    subgraph "工具系统层 (Tool System Layer)"
        I --> U[工具注册表]
        U --> V[文件工具]
        U --> W[Bash工具]
        U --> X[Web工具]
        U --> Y[搜索工具]
        U --> Z[任务工具]
        U --> AA[MCP工具]
        U --> AB[自定义工具]
    end
    
    subgraph "会话管理层 (Session Management Layer)"
        J --> AC[会话管理器]
        AC --> AD[会话存储]
        AC --> AE[会话历史]
        AC --> AF[会话模板]
        J --> AG[对话管理]
        AG --> AH[消息流]
        AG --> AI[状态同步]
    end
    
    subgraph "插件系统层 (Plugin System Layer)"
        K --> AJ[插件管理器]
        AJ --> AK[认证插件]
        AJ --> AL[权限插件]
        AJ --> AM[工具插件]
        AJ --> AN[钩子插件]
        AJ --> AO[MCP插件]
        K --> AP[插件API]
        AP --> AQ[插件接口]
        AP --> AR[插件生命周期]
    end
    
    subgraph "权限管理层 (Permission Management Layer)"
        L --> AS[权限管理器]
        AS --> AT[权限规则]
        AT --> AU[工具权限]
        AT --> AV[命令权限]
        AT --> AW[路径权限]
        AS --> AX[权限检查]
        AX --> AY[权限验证]
        AX --> AZ[权限拦截]
    end
    
    subgraph "状态管理层 (State Management Layer)"
        M --> BA[状态管理器]
        BA --> BB[状态存储]
        BB --> BC[内存存储]
        BB --> BD[文件存储]
        BB --> BE[数据库存储]
        BA --> BF[状态同步]
        BF --> BG[状态快照]
        BF --> BH[状态恢复]
    end
    
    subgraph "数据持久化层 (Data Persistence Layer)"
        AD --> BI[文件系统]
        BD --> BI
        BB --> BI
        BI --> BJ[JSON存储]
        BI --> BK[SQLite存储]
        BI --> BL[配置文件]
    end
    
    subgraph "外部服务层 (External Services Layer)"
        O --> BM[AI API]
        BM --> BN[Anthropic API]
        BM --> BO[OpenAI API]
        BM --> BP[其他AI服务]
        AA --> BQ[MCP服务器]
        BQ --> BR[外部工具]
        BQ --> BS[外部服务]
    end
    
    subgraph "基础设施层 (Infrastructure Layer)"
        BR --> BT[文件系统]
        BR --> BU[网络服务]
        BR --> BV[进程管理]
        BS --> BW[API网关]
        BS --> BX[消息队列]
        BS --> BY[监控系统]
    end
    
    style A fill:#e1f5fe
    style B fill:#e8f5e8
    style C fill:#fff3e0
    style D fill:#f3e5f5
    style E fill:#fce4ec
    style F fill:#f1f8e9
    style G fill:#e0f2f1
    style H fill:#e8eaf6
    style I fill:#fff8e1
    style J fill:#f3e5f5
    style K fill:#e0f2f1
    style L fill:#fce4ec
    style M fill:#f1f8e9
```

## 🔧 详细组件架构图

### 1. 核心组件关系图

```mermaid
graph LR
    subgraph "用户交互"
        A[用户输入] --> B[输入处理器]
        B --> C[命令解析器]
        C --> D[参数验证器]
    end
    
    subgraph "核心引擎"
        D --> E[查询引擎]
        E --> F[工具调度器]
        F --> G[AI客户端]
        G --> H[响应处理器]
        H --> I[输出格式化器]
        I --> J[用户界面]
    end
    
    subgraph "支持系统"
        E --> K[权限管理器]
        E --> L[状态管理器]
        E --> M[插件管理器]
        F --> N[工具注册表]
        G --> O[配置管理器]
        H --> P[日志系统]
    end
    
    subgraph "数据层"
        K --> Q[权限规则库]
        L --> R[状态存储]
        M --> S[插件仓库]
        N --> T[工具定义库]
        O --> U[配置存储]
        P --> V[日志存储]
    end
```

### 2. 工具系统架构图

```mermaid
graph TB
    subgraph "工具接口层"
        A[工具接口] --> B[文件工具接口]
        A --> C[系统工具接口]
        A --> D[网络工具接口]
        A --> E[自定义工具接口]
    end
    
    subgraph "工具实现层"
        B --> F[文件读取工具]
        B --> G[文件写入工具]
        B --> H[文件搜索工具]
        
        C --> I[Bash执行工具]
        C --> J[进程管理工具]
        C --> K[系统信息工具]
        
        D --> L[HTTP客户端工具]
        D --> M[网页抓取工具]
        D --> N[API调用工具]
        
        E --> O[自定义业务工具]
        E --> P[第三方集成工具]
    end
    
    subgraph "工具管理层"
        F --> Q[工具注册表]
        G --> Q
        H --> Q
        I --> Q
        J --> Q
        K --> Q
        L --> Q
        M --> Q
        N --> Q
        O --> Q
        P --> Q
        
        Q --> R[工具调度器]
        R --> S[权限检查器]
        S --> T[执行环境]
        T --> U[结果收集器]
    end
    
    subgraph "工具执行层"
        U --> V[文件系统操作]
        U --> W[系统命令执行]
        U --> X[网络请求处理]
        U --> Y[自定义逻辑执行]
    end
```

### 3. AI引擎架构图

```mermaid
graph TB
    subgraph "AI客户端层"
        A[AI客户端接口] --> B[Anthropic客户端]
        A --> C[OpenAI客户端]
        A --> D[其他AI客户端]
    end
    
    subgraph "查询处理层"
        B --> E[查询构建器]
        C --> E
        D --> E
        
        E --> F[查询验证器]
        F --> G[查询优化器]
        G --> H[查询调度器]
    end
    
    subgraph "响应处理层"
        H --> I[响应解析器]
        I --> J[响应验证器]
        J --> K[响应处理器]
        K --> L[结果格式化器]
    end
    
    subgraph "上下文管理层"
        E --> M[上下文收集器]
        M --> N[上下文压缩器]
        N --> O[上下文管理器]
        O --> P[上下文存储器]
    end
    
    subgraph "记忆管理层"
        H --> Q[记忆收集器]
        Q --> R[记忆压缩器]
        R --> S[记忆管理器]
        S --> T[记忆存储器]
    end
    
    subgraph "流式处理层"
        I --> U[流式处理器]
        U --> V[流式验证器]
        V --> W[流式输出器]
    end
```

### 4. 权限管理架构图

```mermaid
graph TB
    subgraph "权限策略层"
        A[权限策略] --> B[工具权限策略]
        A --> C[命令权限策略]
        A --> D[路径权限策略]
        A --> E[时间权限策略]
    end
    
    subgraph "权限规则层"
        B --> F[工具白名单]
        B --> G[工具黑名单]
        B --> H[工具限制规则]
        
        C --> I[命令白名单]
        C --> J[命令黑名单]
        C --> K[命令限制规则]
        
        D --> L[路径访问规则]
        D --> M[文件类型规则]
        D --> N[目录规则]
        
        E --> O[时间限制规则]
        E --> P[频率限制规则]
    end
    
    subgraph "权限检查层"
        F --> Q[权限检查器]
        G --> Q
        H --> Q
        I --> Q
        J --> Q
        K --> Q
        L --> Q
        M --> Q
        N --> Q
        O --> Q
        P --> Q
        
        Q --> R[权限验证器]
        R --> S[权限决策器]
        S --> T[权限执行器]
    end
    
    subgraph "权限审计层"
        T --> U[权限日志记录器]
        U --> V[权限审计器]
        V --> W[权限报告器]
    end
```

### 5. 插件系统架构图

```mermaid
graph TB
    subgraph "插件接口层"
        A[插件接口] --> B[认证插件接口]
        A --> C[权限插件接口]
        A --> D[工具插件接口]
        A --> E[钩子插件接口]
        A --> F[MCP插件接口]
    end
    
    subgraph "插件实现层"
        B --> G[认证插件实现]
        C --> H[权限插件实现]
        D --> I[工具插件实现]
        E --> J[钩子插件实现]
        F --> K[MCP插件实现]
    end
    
    subgraph "插件管理层"
        G --> L[插件注册表]
        H --> L
        I --> L
        J --> L
        K --> L
        
        L --> M[插件加载器]
        M --> N[插件验证器]
        N --> O[插件初始化器]
        O --> P[插件执行器]
    end
    
    subgraph "插件生命周期层"
        P --> Q[插件启动器]
        Q --> R[插件运行器]
        R --> S[插件监控器]
        S --> T[插件停止器]
        T --> U[插件清理器]
    end
    
    subgraph "插件通信层"
        P --> V[插件消息总线]
        V --> W[插件事件系统]
        W --> X[插件API网关]
        X --> Y[插件服务发现]
    end
```

### 6. 状态管理架构图

```mermaid
graph TB
    subgraph "状态管理层"
        A[状态管理器] --> B[状态收集器]
        A --> C[状态验证器]
        A --> D[状态压缩器]
        A --> E[状态持久化器]
    end
    
    subgraph "状态存储层"
        E --> F[内存存储]
        E --> G[文件存储]
        E --> H[数据库存储]
        E --> I[分布式存储]
    end
    
    subgraph "状态同步层"
        B --> J[状态同步器]
        J --> K[状态冲突解决器]
        K --> L[状态一致性检查器]
        L --> M[状态同步监控器]
    end
    
    subgraph "状态快照层"
        D --> N[状态快照器]
        N --> O[快照存储器]
        O --> P[快照恢复器]
        P --> Q[快照管理器]
    end
    
    subgraph "状态监控层"
        A --> R[状态监控器]
        R --> S[状态指标收集器]
        S --> T[状态健康检查器]
        T --> U[状态报警器]
    end
```

## 🔄 数据流图

### 1. 用户请求处理流程

```mermaid
sequenceDiagram
    participant U as 用户
    participant I as 输入处理器
    participant C as 命令解析器
    participant P as 权限管理器
    participant E as 查询引擎
    participant T as 工具调度器
    participant AI as AI客户端
    participant R as 响应处理器
    participant O as 输出界面
    
    U->>I: 用户输入
    I->>C: 解析输入
    C->>P: 权限检查
    P-->>C: 权限结果
    C->>E: 构建查询
    E->>T: 工具调度
    T->>AI: AI请求
    AI-->>T: AI响应
    T->>R: 处理响应
    R->>O: 格式化输出
    O->>U: 显示结果
```

### 2. 工具执行流程

```mermaid
sequenceDiagram
    participant E as 查询引擎
    participant T as 工具调度器
    participant R as 工具注册表
    participant P as 权限检查器
    participant S as 执行环境
    participant W as 工具执行器
    participant M as 结果收集器
    participant E as 查询引擎
    
    E->>T: 请求工具执行
    T->>R: 查找工具
    R-->>T: 工具定义
    T->>P: 权限检查
    P-->>T: 权限结果
    T->>S: 准备执行环境
    S->>W: 执行工具
    W-->>S: 执行结果
    S->>M: 收集结果
    M-->>E: 返回结果
```

### 3. 插件加载流程

```mermaid
sequenceDiagram
    participant M as 插件管理器
    participant L as 插件加载器
    participant V as 插件验证器
    participant I as 插件初始化器
    participant R as 插件注册表
    participant S as 插件状态管理器
    
    M->>L: 加载插件
    L->>V: 验证插件
    V-->>L: 验证结果
    L->>I: 初始化插件
    I-->>L: 初始化结果
    L->>R: 注册插件
    R->>S: 更新状态
    S-->>M: 完成加载
```

## 📊 系统性能架构图

### 1. 性能监控架构

```mermaid
graph TB
    subgraph "性能数据收集"
        A[性能监控器] --> B[响应时间收集器]
        A --> C[资源使用收集器]
        A --> D[错误率收集器]
        A --> E[吞吐量收集器]
    end
    
    subgraph "性能数据处理"
        B --> F[性能数据分析器]
        C --> F
        D --> F
        E --> F
        
        F --> G[性能指标计算器]
        G --> H[性能阈值检查器]
        H --> I[性能异常检测器]
    end
    
    subgraph "性能优化"
        I --> J[性能优化建议器]
        J --> K[自动优化执行器]
        K --> L[优化效果评估器]
    end
    
    subgraph "性能报告"
        L --> M[性能报告生成器]
        M --> N[性能仪表板]
        N --> O[性能警报系统]
    end
```

### 2. 缓存架构

```mermaid
graph TB
    subgraph "缓存层次"
        A[用户缓存] --> B[内存缓存]
        A --> C[磁盘缓存]
        A --> D[分布式缓存]
        
        B --> E[L1缓存]
        C --> F[L2缓存]
        D --> G[L3缓存]
    end
    
    subgraph "缓存管理"
        E --> H[缓存策略管理器]
        F --> H
        G --> H
        
        H --> I[缓存失效管理器]
        I --> J[缓存更新管理器]
        J --> K[缓存清理管理器]
    end
    
    subgraph "缓存监控"
        K --> L[缓存命中率监控器]
        L --> M[缓存性能监控器]
        M --> N[缓存容量监控器]
    end
```

## 🔐 安全架构图

### 1. 安全防护架构

```mermaid
graph TB
    subgraph "输入安全"
        A[输入验证器] --> B[SQL注入防护]
        A --> C[XSS防护]
        A --> D[命令注入防护]
        A --> E[路径遍历防护]
    end
    
    subgraph "执行安全"
        B --> F[沙箱执行器]
        C --> F
        D --> F
        E --> F
        
        F --> G[资源限制器]
        G --> H[权限隔离器]
        H --> I[行为监控器]
    end
    
    subgraph "数据安全"
        I --> J[数据加密器]
        J --> K[数据脱敏器]
        K --> L[数据访问控制]
    end
    
    subgraph "审计安全"
        L --> M[操作审计器]
        M --> N[安全事件监控器]
        N --> O[安全警报系统]
    end
```

## 🏗️ 部署架构图

### 1. 容器化部署架构

```mermaid
graph TB
    subgraph "容器编排"
        A[Kubernetes集群] --> B[API服务Pod]
        A --> C[前端服务Pod]
        A --> D[数据库Pod]
        A --> E[缓存Pod]
        A --> F[监控Pod]
    end
    
    subgraph "服务网格"
        B --> G[服务网格]
        C --> G
        D --> G
        E --> G
        F --> G
    end
    
    subgraph "数据层"
        D --> H[主数据库]
        E --> I[Redis缓存]
        F --> J[时序数据库]
    end
    
    subgraph "监控层"
        F --> K[日志收集器]
        K --> L[监控代理]
        L --> M[告警系统]
    end
```

### 2. 微服务部署架构

```mermaid
graph TB
    subgraph "API网关"
        A[API网关] --> B[认证服务]
        A --> C[权限服务]
        A --> D[路由服务]
    end
    
    subgraph "核心服务"
        B --> E[AI服务]
        C --> F[工具服务]
        D --> G[会话服务]
    end
    
    subgraph "支持服务"
        E --> H[插件服务]
        F --> I[状态服务]
        G --> J[日志服务]
    end
    
    subgraph "数据服务"
        H --> K[配置服务]
        I --> L[存储服务]
        J --> M[分析服务]
    end
```

---

## 📝 架构说明

### 设计原则

1. **模块化设计**: 系统采用高度模块化的设计，各个组件职责明确，便于维护和扩展。
2. **可扩展性**: 插件系统和工具系统支持动态扩展，可以轻松添加新功能。
3. **安全性**: 多层次的安全防护机制，确保系统运行安全。
4. **性能优化**: 多级缓存和性能监控，确保系统高效运行。
5. **可维护性**: 完善的日志和监控系统，便于问题排查和维护。

### 技术栈

- **后端**: Go 1.25.6+
- **前端**: React 18.3.1 + TypeScript 5.7.3
- **AI集成**: Anthropic SDK, OpenAI SDK
- **数据库**: SQLite (可扩展为PostgreSQL)
- **缓存**: Redis
- **监控**: Prometheus + Grafana
- **容器化**: Docker + Kubernetes

### 架构优势

1. **高性能**: Go语言的并发特性和优化的架构设计确保系统高性能。
2. **可扩展**: 插件系统和微服务架构支持水平扩展。
3. **易维护**: 清晰的模块划分和完善的监控便于维护。
4. **用户友好**: 多种交互模式满足不同用户需求。
5. **安全可靠**: 多层次的安全防护确保系统安全可靠。

---

*架构设计文档版本: v1.0.0*  
*最后更新: 2026-04-09*