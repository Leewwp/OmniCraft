package container

// #672 REQUIRED/OPTIONAL 分类完整性守门：ServiceContainer 的每一个字段
// 都必须归入三类之一——REQUIRED（ValidateWiring 断言非 nil）、OPTIONAL
// （缺席不进缺失清单，须写明原因）、非依赖字段（状态/回执，不是注入面）。
// 新增依赖字段若未分类，本测试直接红；分类与 ValidateWiring 清单做双向
// 集合相等校验——多断言一个未分类依赖、或分类了却漏断言，都过不了。
// census 口径（#672 时点）：83 个字段 = 76 REQUIRED + 6 OPTIONAL + 1 非依赖；
// 数量随字段增减浮动，由集合相等断言兜底，不硬编码总数。

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

type fieldClass int

const (
	classRequired fieldClass = iota
	classOptional
	classNotDependency
)

// containerFieldClass 登记单个字段的分类；REQUIRED 项必须带 wiring 名
// （ValidateWiring 清单里的点分名），其余两类写明原因。
type containerFieldClass struct {
	class      fieldClass
	wiringName string // REQUIRED 专用
	reason     string // OPTIONAL / 非依赖专用
}

// containerFieldClasses 是唯一分类台账。改 ServiceContainer 结构的 PR
// 必须同步改这里（或证明新字段属于既有 OPTIONAL 语义）。
var containerFieldClasses = map[string]containerFieldClass{
	// ---- inputs（组合根三输入 + 队列面）----
	"DB":            {classRequired, "db", ""},
	"RDB":           {classRequired, "redis", ""},
	"Cfg":           {classRequired, "config", ""},
	"QueueBroker":   {classOptional, "", "queue 关闭时为 nil；消费侧只经 StartWorkers 且自带 nil 守卫"},
	"QueueProducer": {classRequired, "queue.producer", ""},

	// ---- repositories（全部 REQUIRED）----
	"UserRepo":           {classRequired, "repo.user", ""},
	"ContentRepo":        {classRequired, "repo.content", ""},
	"IPRepo":             {classRequired, "repo.ip", ""},
	"SocialRepo":         {classRequired, "repo.social", ""},
	"FollowRepo":         {classRequired, "repo.follow", ""},
	"JudgeRepo":          {classRequired, "repo.judge", ""},
	"TagRepo":            {classRequired, "repo.tag", ""},
	"CategoryRepo":       {classRequired, "repo.category", ""},
	"PRRepo":             {classRequired, "repo.pr", ""},
	"VersionRepo":        {classRequired, "repo.version", ""},
	"AppealRepo":         {classRequired, "repo.appeal", ""},
	"NotificationRepo":   {classRequired, "repo.notification", ""},
	"BrowseHistoryRepo":  {classRequired, "repo.browse_history", ""},
	"DiscussionRepo":     {classRequired, "repo.discussion", ""},
	"MessageRepo":        {classRequired, "repo.message", ""},
	"RehabRepo":          {classRequired, "repo.rehab", ""},
	"EmbeddingRepo":      {classRequired, "repo.embedding", ""},
	"LLMConfigRepo":      {classRequired, "repo.llm_config", ""},
	"SearchRepo":         {classRequired, "repo.search", ""},
	"FeedbackRepo":       {classRequired, "repo.feedback", ""},
	"AdminAuditRepo":     {classRequired, "repo.admin_audit", ""},
	"OutboxRepo":         {classRequired, "repo.outbox", ""},
	"ArchiveScanRepo":    {classRequired, "repo.archive_scan", ""},
	"OpenSearchRepo":     {classRequired, "opensearch.repo", ""},
	"HybridRetriever":    {classRequired, "hybrid.retriever", ""},
	"AgentTraceRepo":     {classRequired, "repo.agent_trace", ""},
	"AgentTraceWriter":   {classRequired, "agenttrace.writer", ""},
	"RagEvaluationRepo":  {classRequired, "repo.rag_evaluation", ""},
	"CollectionRepo":     {classRequired, "repo.collection", ""},
	"SeriesRepo":         {classRequired, "repo.series", ""},
	"IPVisitHistoryRepo": {classRequired, "repo.ip_visit_history", ""},

	// ---- services ----
	"AuthService":          {classRequired, "service.auth", ""},
	"VerificationService":  {classRequired, "service.verification", ""},
	"ContentService":       {classRequired, "service.content", ""},
	"StudioContentService": {classRequired, "service.studio_content", ""},
	// OSS 可选三件（#672 显式分类）：presign 服务缺席时各面保持
	// fail-open 503；init 错误回执不是注入面。
	"OSSService":          {classOptional, "", "OSS 未配置部署保持各面 fail-open 503"},
	"OSSInitErr":          {classNotDependency, "", "NewOSSService 的错误回执（状态位），非注入依赖"},
	"UploadGrants":        {classRequired, "upload.grants", ""},
	"IPService":           {classRequired, "service.ip", ""},
	"IPAdminService":      {classRequired, "service.ip_admin", ""},
	"LLMConfigService":    {classRequired, "service.llm_config", ""},
	"DLQWorker":           {classOptional, "", "随非 nil redis 派生（redis 本身 REQUIRED）；自身不入清单"},
	"IPPublishService":    {classRequired, "service.ip_publish", ""},
	"AgentQuotaReserver":  {classRequired, "agent.quota_reserver", ""},
	"DownloadArchiveGate": {classRequired, "gate.download_archive", ""},
	"CategoryService":     {classRequired, "service.category", ""},
	"CollectionService":   {classRequired, "service.collection", ""},
	"SeriesService":       {classRequired, "service.series", ""},
	"TagService":          {classRequired, "service.tag", ""},
	"RehabService":        {classRequired, "service.rehab", ""},
	"SocialService":       {classRequired, "service.social", ""},
	"ReputationService":   {classRequired, "service.reputation", ""},
	"ReviewService":       {classRequired, "service.review", ""},
	"JudgeService":        {classRequired, "service.judge", ""},
	"RecommendationSvc":   {classRequired, "service.recommendation", ""},
	"StatsService":        {classRequired, "service.stats", ""},
	"IPStatsService":      {classRequired, "service.ip_stats", ""},
	"AgentService":        {classRequired, "service.agent", ""},
	"AgentTokenService":   {classRequired, "service.agent_token", ""},
	"NotificationService": {classRequired, "service.notification", ""},
	"PRService":           {classRequired, "service.pr", ""},
	"VersionService":      {classRequired, "service.version", ""},
	"UsageGuideService":   {classRequired, "service.usage_guide", ""},
	"MCPHandler":          {classRequired, "mcp.handler", ""},
	"SearchService":       {classRequired, "service.search", ""},
	"IPProposalService":   {classRequired, "service.ip_proposal", ""},
	"FeedbackService":     {classRequired, "service.feedback", ""},
	"AdminAuditService":   {classRequired, "service.admin_audit", ""},
	"CollabInviteService": {classRequired, "service.collab_invite", ""},
	"CaptchaVerifier":     {classRequired, "captcha.verifier", ""},
	"CaptchaProvider":     {classRequired, "captcha.provider", ""},
	"CaptchaTickets":      {classRequired, "captcha.tickets", ""},
	"RAGProjection":       {classRequired, "rag.projection", ""},
	// 归档扫描可选二件：feature 门（archive_malware_scan_enabled）后置。
	"ArchiveObjectStore": {classOptional, "", "features.archive_malware_scan_enabled 关闭时不构造"},
	"ArchiveScanner":     {classOptional, "", "同上，与 ArchiveObjectStore 同门"},
	// prompt registry：断言面是 resolver（下方 REQUIRED）；repo 随 db
	// 无条件构造，seed 失败也只降级 builtin，不入清单。
	"PromptRegistryRepo":    {classOptional, "", "随 db 无条件构造；被断言面是 service.prompt_registry"},
	"PromptRegistryService": {classRequired, "service.prompt_registry", ""},
	"DisplayURLSigner":      {classRequired, "display.signer", ""},
}

// TestServiceContainerFieldsFullyClassified：反射全字段对账台账——
// 新依赖字段未归类即失败；台账里的陈旧条目（字段已删）同样失败。
func TestServiceContainerFieldsFullyClassified(t *testing.T) {
	structType := reflect.TypeOf(ServiceContainer{})
	for i := 0; i < structType.NumField(); i++ {
		name := structType.Field(i).Name
		class, ok := containerFieldClasses[name]
		if !ok {
			t.Errorf("ServiceContainer 新字段 %s 未分类：请在 containerFieldClasses 登记 REQUIRED/OPTIONAL/非依赖（#672 守门）", name)
			continue
		}
		switch class.class {
		case classRequired:
			if class.wiringName == "" {
				t.Errorf("字段 %s 标记 REQUIRED 但缺 wiring 名", name)
			}
		case classOptional, classNotDependency:
			if class.reason == "" {
				t.Errorf("字段 %s 标记 OPTIONAL/非依赖 但缺原因说明", name)
			}
		}
	}
	structFields := make(map[string]bool, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		structFields[structType.Field(i).Name] = true
	}
	for name := range containerFieldClasses {
		if !structFields[name] {
			t.Errorf("分类台账含已删除的字段 %s：请清理陈旧条目", name)
		}
	}
}

// TestValidateWiringRequiredSetMatchesClassification：零值容器的缺失
// 清单（= ValidateWiring 的 REQUIRED 全集）与台账 REQUIRED 分类做双向
// 集合相等。缺失报告仍须一次列出全部项（清单语义由本测试钉死，不再
// 依赖 wiring_test.go 的 ≥60 数量级启发式）。
func TestValidateWiringRequiredSetMatchesClassification(t *testing.T) {
	err := (&ServiceContainer{}).ValidateWiring()
	if err == nil {
		t.Fatal("zero-value container must fail ValidateWiring")
	}
	msg := err.Error()
	const marker = "missing REQUIRED dependencies: "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		t.Fatalf("missing-dependency list not found in: %s", msg)
	}
	asserted := strings.Split(msg[idx+len(marker):], ", ")
	sort.Strings(asserted)

	var classified []string
	for name, class := range containerFieldClasses {
		if class.class == classRequired {
			if class.wiringName == "" {
				t.Errorf("字段 %s 分类 REQUIRED 但无 wiring 名", name)
				continue
			}
			classified = append(classified, class.wiringName)
		}
	}
	sort.Strings(classified)

	if len(asserted) == len(classified) {
		for i := range asserted {
			if asserted[i] != classified[i] {
				t.Fatalf("ValidateWiring 清单与分类台账不一致:\n  断言: %v\n  分类: %v", asserted, classified)
			}
		}
		return
	}
	t.Fatalf("ValidateWiring 清单与分类台账不一致（双向集合不等）:\n  断言(%d): %v\n  分类(%d): %v",
		len(asserted), asserted, len(classified), classified)
}
