# 阿里云内容安全（Green）媒体审核回调调研

> **创建日期**: 2026-08-08  
> **预计失效日期**: 2026-10-08  
> **调研范围**: 仅使用阿里云帮助中心、阿里云 OpenAPI 文档/元数据作为事实来源；未使用博客、论坛或第三方资料。  
> **核对日期**: 2026-08-08

## 结论速览

1. 阿里云内容安全的异步媒体审核支持“回调”或“轮询”两条结果获取路径。旧版 Green API 的图片/视频异步接口在请求中传入 `callback`、`seed`，并可传 `cryptType`；当前内容安全增强版的视频文件审核文档也明确采用同样的请求参数模式。
2. 回调的官方认证机制是 `checksum`：将阿里云账号 UID、用户自定义 `seed`、回调 `content` 拼接后按指定算法计算，再由业务服务端校验。官方文档没有把来源 IP/CIDR 白名单列为回调接口的必需参数或必配项。
3. `GREEN_CALLBACK_ALLOWED_IPS` 不是阿里云 API 参数名，也不是本次核对的阿里云内容安全控制台字段。若应用自行维护该变量，它应被理解为应用侧的额外入站访问控制，而不是“阿里云控制台配置”。
4. 不能仅依据阿里云官方材料把 `114.115.0.0/16` 等网段认定为 Green 回调官方来源网段：本次核对的官方回调文档、视频 API 文档和 OpenAPI 元数据均未发布该网段或要求按该网段放行。
5. 控制台确实存在回调相关配置，但文档描述的是机器审核/人工审核的“消息通知”方案（回调地址、加密算法、通知类型、审核结果，并由系统生成 `seed`）；异步扫描结果回调则要求调用方准备地址和 `seed`，在 API 请求中传入 `callback` 和 `seed`。

## 1. 官方 API 的媒体审核范围

### 1.1 Green 1.0（旧版 `/green/...` API）

阿里云官方 API 概览列出以下内容审核接口：

- 图片：`/green/image/scan`（同步）、`/green/image/asyncscan`（异步）、`/green/image/results`（异步结果查询）、`/green/image/feedback`（结果反馈）。
- 视频：`/green/video/syncscan`（同步）、`/green/video/asyncscan`（异步）、`/green/video/results`（异步结果查询）、`/green/video/cancelscan`（取消视频流检测）、`/green/video/feedback`（结果反馈）。
- 文本：`/green/text/scan`（同步）和 `/green/text/feedback`。

来源：阿里云官方《API概览》：[内容安全 API 概览](https://help.aliyun.com/zh/document_detail/70409.html)。

### 1.2 当前内容安全增强版 OpenAPI（Green 2022-03-02）

阿里云 OpenAPI 官方概览当前列出图片、语音、视频、文件及多模态审核接口，包括：`ImageModeration`、`ImageAsyncModeration`、`DescribeImageModerationResult`、`VoiceModeration`、`VoiceModerationResult`、`VideoModeration`、`VideoModerationResult`、`FileModeration`、`DescribeFileModerationResult`、`MultiModalGuardAsync` 和对应结果查询接口。

来源：阿里云 OpenAPI 官方：[Green 2022-03-02 API 概览](https://next.api.aliyun.com/document/Green/2022-03-02/overview)。

当前视频文件审核增强版文档明确说明：`VideoModeration` 只提交异步检测任务，结果通过 `callback` 或轮询 `VideoModerationResult` 获取，结果最长保留 24 小时。

来源：阿里云帮助中心官方《视频文件审核增强版 API》：[2505810](https://help.aliyun.com/zh/document_detail/2505810.html)。

## 2. 异步回调的官方要求

### 2.1 回调地址和传输格式

官方消息通知文档规定，回调地址应满足以下要求：

- 是 HTTP 或 HTTPS 协议、可从公网访问的 URL；
- 支持 POST；
- 支持 UTF-8；
- 接收格式为 `application/x-www-form-urlencoded`；
- 支持表单参数 `checksum` 和 `content`。

来源：阿里云帮助中心官方《如何在内容安全中配置消息通知》：[130313](https://help.aliyun.com/zh/document_detail/130313.html)。视频异步 API 对回调地址的同一要求见：[视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html) 和 [视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)。

### 2.2 `callback`、`seed`、`cryptType`

旧版 Green 视频异步接口的官方参数说明为：

- `callback` 可选；传入后由内容安全向该 URL 推送检测结果；不传则必须轮询；
- 使用 `callback` 时必须提供 `seed`；`seed` 由调用方自定义，允许英文字母、数字和下划线，长度不超过 64；
- `cryptType` 用于设置回调签名/加密算法，默认 `SHA256`，也支持 `SM3`。

来源：[视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html)（“异步检测”请求参数中的 `callback`、`seed`、`cryptType`）。

当前视频文件审核增强版同样把 `callback`、`seed`、`cryptType` 放在 `ServiceParameters` 的参数说明中：`callback` 为空时轮询；`seed` 在使用回调时必填；`cryptType` 支持默认 SHA256 或 SM3。

来源：[视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)。

### 2.3 `checksum` 如何校验

官方文档给出的校验规则是：

```text
checksum = SHA256(<用户 UID> + <seed> + <content>)
```

其中：

- UID 是阿里云账号 UID，不是 RAM 用户 UID；
- `content` 是字符串形式的 JSON，需要业务服务端自行解析；
- 服务端收到推送后，应按同一规则重新计算并与 `checksum` 比对，以校验请求来源/防篡改；
- 新版参数说明还明确允许将算法设置为 `SM3`；SM3 返回小写十六进制字符串。

来源：[视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html)、[视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)、[如何在内容安全中配置消息通知 130313](https://help.aliyun.com/zh/document_detail/130313.html)。

### 2.4 ACK 和失败重试

接口级视频文档写明：回调接口返回 HTTP 200 表示接收成功，其他状态码视为失败；视频异步接口文档写明失败时最多重复推送 16 次，16 次后停止推送。[70436](https://help.aliyun.com/zh/document_detail/70436.html) 和当前增强版视频文档 [2505810](https://help.aliyun.com/zh/document_detail/2505810.html) 都有该表述。

同时，通用消息通知文档 [130313](https://help.aliyun.com/zh/document_detail/130313.html) 的“回调次数”概念表写的是最多重复推送 3 次。两者是阿里云官方文档之间的差异；实现重试/幂等时不应把 3 或 16 当成跨所有回调类型的统一保证，应该按实际使用的 API 和当前产品文档确认，并始终按可能重复投递设计。

## 3. 来源 IP、签名和白名单

### 3.1 官方明确要求的来源认证

阿里云官方材料明确描述的是 `seed` + `checksum` 校验：`seed` 用于校验请求是否来自内容安全服务端，`checksum` 用于按 UID、seed、content 验证并防篡改。

来源：[如何在内容安全中配置消息通知 130313](https://help.aliyun.com/zh/document_detail/130313.html)；[视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html)。

### 3.2 本次官方材料中未发现的要求

在本次核对的以下官方来源中，未发现“回调来源 IP/CIDR 固定网段”“回调来源 IP 白名单”“在控制台录入回调来源 IP”或类似要求：

- [内容安全消息通知 130313](https://help.aliyun.com/zh/document_detail/130313.html)；
- [视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html)；
- [视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)；
- [Green 2022-03-02 的 VideoModeration OpenAPI 文档](https://next.api.aliyun.com/document/Green/2022-03-02/VideoModeration)；
- [VideoModeration 官方 OpenAPI 元数据](https://next.api.aliyun.com/meta/v1/products/Green/versions/2022-03-02/apis/VideoModeration/api)。

因此，基于官方公开文档可以确认：公网可达是回调地址要求；固定来源 IP 白名单不是这些文档列出的官方接入必需项。由于这是对公开官方材料的核对结果，不应扩大解释为阿里云在所有未公开内部系统中绝对不存在任何网络策略。

### 3.3 关于 `GREEN_CALLBACK_ALLOWED_IPS`

结论：`GREEN_CALLBACK_ALLOWED_IPS` 不是阿里云 Green API 的官方参数名，也不是上述官方文档描述的控制台配置项。阿里云 API 使用的是 `callback`、`seed`、`cryptType`、`checksum` 等名称；当前 `VideoModeration` 的官方 OpenAPI 元数据也只将请求入口描述为 `Service` 与 `ServiceParameters`，没有该字段。

来源：[Green 2022-03-02 VideoModeration API 元数据](https://next.api.aliyun.com/meta/v1/products/Green/versions/2022-03-02/apis/VideoModeration/api)；[视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)。

如果应用配置了 `GREEN_CALLBACK_ALLOWED_IPS`，它只能被归类为应用/部署方自行维护的入站 IP/CIDR allowlist。它不是“需要在阿里云内容安全控制台填写的配置”，也不能从阿里云官方文档推出其默认值或官方网段。尤其不能仅凭该变量名或非官方材料把 `114.115.0.0/16` 作为阿里云 Green 回调来源网段。

## 4. 哪些内容属于阿里云控制台配置

官方文档区分了两类回调：

### 4.1 异步扫描结果回调：请求级参数

对于图片/视频等内容检测 API 的异步扫描结果回调，调用方需要：

1. 自行准备可公网访问的 HTTP 回调地址和 `seed`；
2. 调用异步 API 时传入 `callback` 和 `seed`；
3. 按 `checksum` 规则校验回调。

来源：[如何在内容安全中配置消息通知 130313](https://help.aliyun.com/zh/document_detail/130313.html) 的“扫描结果回调通知”；[视频审核异步检测 70436](https://help.aliyun.com/zh/document_detail/70436.html)。

这意味着对该路径而言，回调 URL、seed 和算法选择是 API 请求/业务接入的一部分，而不是一个名为 `GREEN_CALLBACK_ALLOWED_IPS` 的阿里云控制台白名单字段。

### 4.2 机器审核/人工审核消息通知：控制台方案

对于“人机审核”场景，官方操作步骤要求进入内容安全控制台，在“机器审核 V1.0 > 设置规则 > 机器审核 > 消息通知”中新增通知，填写方案名称和回调地址，选择加密算法、通知类型、审核结果；保存后系统自动生成 `seed`，再在业务场景管理中关联该消息通知方案。

来源：[如何在内容安全中配置消息通知 130313](https://help.aliyun.com/zh/document_detail/130313.html)。

这是控制台的回调方案配置，但官方步骤仍未要求配置回调来源 IP 白名单。另一个官方视频增强版文档说明，视频审核的截帧方式、频率、画面/语音检测规则和结果返回范围可在内容安全控制台配置；这属于审核规则配置，不是回调来源 IP 配置。

来源：[视频文件审核增强版 API 2505810](https://help.aliyun.com/zh/document_detail/2505810.html)。

## 5. 对应用接入的直接判断

| 问题 | 官方材料支持的判断 |
|---|---|
| 是否有媒体异步审核？ | 有。Green 1.0 提供图片/视频异步接口；Green 2022-03-02 提供图片异步、视频异步、语音、文件和多模态相关 API。来源：[API 概览](https://help.aliyun.com/zh/document_detail/70409.html)、[Green OpenAPI 概览](https://next.api.aliyun.com/document/Green/2022-03-02/overview) |
| 是否支持异步回调？ | 支持。回调与轮询均为官方文档描述的结果获取方式。来源：[70436](https://help.aliyun.com/zh/document_detail/70436.html)、[2505810](https://help.aliyun.com/zh/document_detail/2505810.html) |
| 回调如何认证？ | 用 `seed` 参与 `checksum` 计算；可按文档选择 SHA256/SM3。来源：[130313](https://help.aliyun.com/zh/document_detail/130313.html)、[70436](https://help.aliyun.com/zh/document_detail/70436.html) |
| 阿里云是否要求回调来源 IP 白名单？ | 本次核对的官方 API/帮助文档未列出该要求，也未公布 Green 回调固定来源 CIDR；官方明确要求的是公网可达和 checksum 校验。来源：[130313](https://help.aliyun.com/zh/document_detail/130313.html)、[2505810](https://help.aliyun.com/zh/document_detail/2505810.html) |
| `GREEN_CALLBACK_ALLOWED_IPS` 是否是阿里云控制台配置？ | 不是官方 API 参数，也不是本次核对的官方控制台字段；若存在，应视为应用自行维护的 allowlist。来源：[VideoModeration 元数据](https://next.api.aliyun.com/meta/v1/products/Green/versions/2022-03-02/apis/VideoModeration/api)、[130313](https://help.aliyun.com/zh/document_detail/130313.html) |

## 6. 接入建议（推断，不冒充阿里云要求）

- 服务端应优先实现 `checksum` 校验、重复回调幂等和 HTTP 200 ACK；这些是直接由官方回调协议支持的接入行为。
- 如果应用额外保留 `GREEN_CALLBACK_ALLOWED_IPS`，应在配置和部署文档中明确标注“应用侧防御性 allowlist”，不能标注为“阿里云官方回调 IP”。
- 在没有阿里云官方来源网段文档、控制台显示或工单确认前，不应把某个固定 CIDR 写成官方事实；否则可能因阿里云回调出口变化导致合法回调被拒绝。
- 由于官方通用通知文档和视频接口文档对重试次数存在差异，应用应按重复投递处理，而不是依赖固定次数作为业务正确性条件。
