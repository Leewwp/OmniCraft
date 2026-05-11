#!/bin/bash
# OmniCraft Test Data Seeder
# Creates IPs, original content, and fanworks for UI testing

BASE="http://localhost:8080/api/v1"
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo0Miwicm9sZSI6InVzZXIiLCJzdWIiOiJhY2Nlc3MiLCJleHAiOjE3Nzg0MTU0NjUsImlhdCI6MTc3ODQwODI2NX0.Bh9MgpjgWjfs-QBdGnGqECMmh--FxLbdhdenYcs-0s8"
AUTH="Authorization: Bearer $TOKEN"
CT="Content-Type: application/json"

echo "=== Creating IPs ==="

# IP 1: 原神 (Gaming)
IP1=$(curl -s -X POST "$BASE/ips" -H "$AUTH" -H "$CT" -d '{
  "name": "原神",
  "description": "《原神》是米哈游开发的开放世界冒险游戏，以其精美的画面、丰富的角色和广阔的世界观吸引了全球数千万玩家。",
  "category": "gaming",
  "tags": ["开放世界","RPG","米哈游","提瓦特"]
}')
IP1_ID=$(echo "$IP1" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created IP 原神: id=$IP1_ID"

# IP 2: 崩坏星穹铁道 (Gaming)
IP2=$(curl -s -X POST "$BASE/ips" -H "$AUTH" -H "$CT" -d '{
  "name": "崩坏：星穹铁道",
  "description": "米哈游旗下科幻题材回合制RPG，以星际旅行和银河冒险为主题。",
  "category": "gaming",
  "tags": ["回合制","RPG","科幻","米哈游","银河"]
}')
IP2_ID=$(echo "$IP2" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created IP 星穹铁道: id=$IP2_ID"

# IP 3: 鬼灭之刃 (Anime)
IP3=$(curl -s -X POST "$BASE/ips" -H "$AUTH" -H "$CT" -d '{
  "name": "鬼灭之刃",
  "description": "吾峠呼世晴创作的少年漫画，讲述了灶门炭治郎为救妹妹祢豆子而加入鬼杀队的故事。",
  "category": "anime",
  "tags": ["少年漫画","热血","和风","ufotable"]
}')
IP3_ID=$(echo "$IP3" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created IP 鬼灭之刃: id=$IP3_ID"

# IP 4: 流浪地球 (Film/TV)
IP4=$(curl -s -X POST "$BASE/ips" -H "$AUTH" -H "$CT" -d '{
  "name": "流浪地球",
  "description": "刘慈欣原著、郭帆导演的科幻电影系列，讲述人类推动地球逃离太阳系的宏大故事。",
  "category": "film_tv",
  "tags": ["科幻","电影","刘慈欣","国产科幻"]
}')
IP4_ID=$(echo "$IP4" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created IP 流浪地球: id=$IP4_ID"

# IP 5: 明日方舟 (Gaming)
IP5=$(curl -s -X POST "$BASE/ips" -H "$AUTH" -H "$CT" -d '{
  "name": "明日方舟",
  "description": "鹰角网络开发的策略塔防手游，以独特的末日废土世界观和丰富的角色设定闻名。",
  "category": "gaming",
  "tags": ["塔防","策略","末世","兽耳"]
}')
IP5_ID=$(echo "$IP5" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created IP 明日方舟: id=$IP5_ID"

echo ""
echo "=== Creating Original Contents ==="

# Original 1: 影视 - 图文
OC1=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "《流浪地球3》首批剧照深度解析：行星发动机设计美学",
  "description": "## 发动机的工业美学\n\n最近《流浪地球3》放出了首批官方剧照，行星发动机终于有了正面特写。这次的设计语言明显更偏向**重工业科幻**风格——不再是单纯的大就是美，而是有了更精细的机械结构展现。\n\n![发动机全景](https://placehold.co/800x450/1a1a2e/e0e0e0?text=Planetary+Engine)\n\n### 设计细节\n\n1. **进气口格栅**看起来参考了航空发动机的涡轮结构\n2. 外壳的铆钉排列明显参考了苏联重工业时代的设计\n3. 整体配色从之前的纯金属色变成了带有**蓝色能量光纹**的深灰\n\n> \"我们想要一种既有工业力量感，又不会让人觉得遥不可及的设计\" —— 美术指导在采访中说道\n\n### 个人感受\n\n这几张剧照让我对第三部更有信心了。第一部偏重家国情怀，第二部强调了集体主义，第三部看起来会更加聚焦于**个体在宏大叙事中的挣扎**。\n\n你们觉得新设计怎么样？欢迎在评论区讨论！",
  "zone": "original",
  "category": "film_tv",
  "content_type": "image",
  "tags": ["流浪地球","科幻电影","概念设计","影视分析"]
}')
OC1_ID=$(echo "$OC1" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 流浪地球剧照分析: id=$OC1_ID"

# Original 2: 游戏 - 图文
OC2=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "《黑神话：悟空》隐藏BOSS「青背龙」无伤打法全攻略",
  "description": "## 前言\n\n花了整整三天时间研究青背龙的每一个出招前摇，终于做到了无伤通关。这篇攻略会详细拆解每个阶段你需要知道的一切。\n\n### 装备推荐\n\n| 装备 | 推荐理由 |\n|------|----------|\n| 业火杖 | 火属性克制龙类 |\n| 锦斓袈裟 | 高雷抗+自动回血 |\n| 定风珠 | 免疫龙卷风控场 |\n\n### 一阶段：雷电形态\n\n青背龙的雷属性攻击前摇非常短，**判定窗口只有0.3秒**。关键是要看它左爪的姿势——\n- 左爪高举 → 落雷（侧闪）\n- 左爪横扫 → 雷刃（后跳）\n- 双爪合拢 → 全屏雷暴（定风珠打断）\n\n### 二阶段：狂风形态（HP < 40%）\n\n这是最容易翻车的阶段。它会开始召唤龙卷风限制走位，然后用冲刺攻击把你逼到角落。\n\n**关键策略**：保持场地中央，用定风珠免疫龙卷风，等它冲刺后的0.8秒硬直输出。\n\n祝各位天命人武运昌隆！🐒",
  "zone": "original",
  "category": "gaming",
  "content_type": "image",
  "tags": ["黑神话悟空","BOSS攻略","动作游戏","国产游戏"]
}')
OC2_ID=$(echo "$OC2" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 黑神话攻略: id=$OC2_ID"

# Original 3: 美食 - 图文
OC3=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "复刻《孤独的美食家》五郎同款「蒜香黄油煎饭团」",
  "description": "## 五郎叔吃了都说好 🍙\n\n昨晚重温《孤独的美食家》第九季，看到五郎在一家小居酒屋吃蒜香黄油煎饭团的那一幕，馋得不行！今天立刻复刻了一版。\n\n### 食材准备\n\n- 冷米饭 300g（隔夜饭最佳）\n- 黄油 20g\n- 大蒜 4瓣（切薄片）\n- 酱油 1大勺\n- 味淋 1小勺\n- 海苔 2片\n- 白芝麻 适量\n\n### 步骤\n\n1. 冷米饭捏成三角形饭团，**捏紧实**但不捏碎米粒\n2. 平底锅中小火融化黄油，加入蒜片煸至金黄\n3. 饭团入锅，两面各煎3分钟至表面焦脆\n4. 酱油+味淋混合刷在饭团表面，再煎30秒上色\n5. 包上海苔，撒白芝麻\n\n![成品图](https://placehold.co/800x600/F5F5DC/333?text=Garlic+Butter+Rice+Ball)\n\n外酥内软，黄油的奶香混着蒜香，酱油的焦香在嘴里爆开——**真的太好吃了！**",
  "zone": "original",
  "category": "food",
  "content_type": "image",
  "tags": ["美食复刻","日料","饭团","孤独的美食家"]
}')
OC3_ID=$(echo "$OC3" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 美食饭团: id=$OC3_ID"

# Original 4: 宠物 - 图文
OC4=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "捡到一只流浪橘猫的第30天：从「纸片猫」到「煤气罐」的蜕变",
  "description": "## Day 1\n\n在小区垃圾桶旁边发现它的时候，瘦得只剩一把骨头，毛都打结了。带去医院检查，医生说大概3个月大，严重营养不良。\n\n![Day1](https://placehold.co/600x400/FFF8DC/333?text=Day+1+Tiny+Orange)\n\n## Day 7\n\n开始主动进食了！虽然还是怕人，但至少不再躲在沙发底下。\n\n## Day 15\n\n第一次主动蹭我的手！！老母亲落泪了 😭\n\n## Day 30\n\n好家伙，谁能想到一个月前那个小可怜，现在已经是能霸占整个沙发的「主子」了？今天称了一下——**胖了整整1.8kg！**\n\n![Day30](https://placehold.co/600x400/FFF8DC/333?text=Day+30+Chonky+Orange)\n\n名字取好了，叫「八筒」——因为它鼻子上有个八字花纹。欢迎来到新家，八筒！🧡",
  "zone": "original",
  "category": "pet",
  "content_type": "image",
  "tags": ["橘猫","流浪猫救助","养猫日记","猫咪"]
}')
OC4_ID=$(echo "$OC4" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 橘猫日记: id=$OC4_ID"

# Original 5: 文学 - 纯文字
OC5=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "短篇科幻 | 《最后的信使》",
  "description": "## 一\n\n2147年的太阳和2025年的没有区别。\n\n至少看起来如此。\n\n林远站在月球基地的全景窗前，看着那颗距离他38万公里的恒星。它还是那么亮，那么温暖——至少在地球上的人看来是这样。\n\n\"最后的传输窗口还有三分钟\"，AI助理的声音在他头盔里响起，\"信使号已就位，目标：比邻星b。\"\n\n林远深吸一口气。他面前的屏幕上是一封未完成的信。\n\n## 二\n\n给未来的人类：\n\n如果你们收到这封信，说明我们失败了。\n\n我不知道该怎么开头。我们在月球背面发现的那个东西——我们叫它「信标」——在激活之后，用了不到48小时就瓦解了地球的整个通讯网络。\n\n不是攻击。更像是……覆盖。\n\n就像你把一张新的桌布铺在旧桌布上。我们的电磁波、无线电、卫星信号——全部消失了，取而代之的是一种我们无法理解的信号模式。\n\n## 三\n\n有人说是外星文明。有人说是我们自己造的AI失控了。\n\n但我不觉得是任何一方。\n\n我觉得信标是一面镜子。它不是在发送什么新的东西，而是在反射我们自己的信号——只是调转了方向。所有的信息、所有的数据、所有的声音，都在被它吸进去，然后……吞掉。\n\n我们不再能互相听到了。\n\n## 四\n\n最后的传输窗口关闭了。\n\n林远按下发送键。他不知道这封信能否到达比邻星，或者在经过了信标的范围后，它是否会变成一串毫无意义的噪声。\n\n但他还是写了。\n\n因为总要有人留下点什么。\n\n总要有人说——\n\n我们在这里过。",
  "zone": "original",
  "category": "literature",
  "content_type": "article",
  "tags": ["科幻","短篇","原创小说","末世"]
}')
OC5_ID=$(echo "$OC5" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 科幻短篇: id=$OC5_ID"

# Original 6: 旅行 - 图文
OC6=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "自驾318川藏线全攻略：从成都到拉萨的2000公里",
  "description": "## 路线概览\n\n历时14天，自驾川藏南线（G318），从成都出发经康定、理塘、稻城、林芝最终抵达拉萨。\n\n### 关键数据\n\n- 总里程：2140km\n- 最高海拔：东达山垭口 5130m\n- 累计爬升：约 35000m\n- 加油费用：约 ¥2800\n- 住宿费用：约 ¥4200\n\n### Day 1-3 成都 → 康定 → 理塘\n\n前三天是适应期。折多山是第一道考验——海拔4298米，很多人在这里开始高反。\n\n**TIPS**：康定出发前一定加满油！新都桥到理塘之间加油站非常少。\n\n![折多山](https://placehold.co/800x450/87CEEB/333?text=Zheduo+Mountain+4298m)\n\n### Day 4-6 稻城亚丁\n\n亚丁三神山是在太壮观了！央迈勇的雪峰在晨光中呈现金色倒影，绝对是此生必看的景色之一。\n\n### Day 7-10 理塘 → 林芝\n\n怒江72拐是整个行程最刺激的路段。连续的发卡弯加上海拔的急剧变化，对驾驶技术和车辆都是考验。\n\n### Day 11-14 林芝 → 拉萨\n\n最后一段路反而是最轻松的。林芝到拉萨的高速已经通车了，但建议走老318国道——尼洋河的风光不会让你后悔。\n\n> 布达拉宫出现在地平线上的那一刻，所有的疲惫都值得了。",
  "zone": "original",
  "category": "travel",
  "content_type": "image",
  "tags": ["川藏线","自驾游","318国道","旅行攻略"]
}')
OC6_ID=$(echo "$OC6" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 川藏线攻略: id=$OC6_ID"

# Original 7: 数码科技 - 视频
OC7=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "MacBook Pro M5 Max 深度评测：性能怪兽还是续航之王？",
  "description": "## 开箱第一印象\n\n新的深空黑色真的太高级了，完全不会留指纹！\n\n### 跑分数据\n\n| 项目 | M5 Max | M4 Max | 提升 |\n|------|--------|--------|------|\n| Geekbench 单核 | 4210 | 3850 | +9.4% |\n| Geekbench 多核 | 26800 | 22100 | +21.3% |\n| Cinebench GPU | 18200 | 15100 | +20.5% |\n\n### 实际体验\n\n- **视频剪辑**：8K ProRes 422 实时预览无压力，6轨同时播放不掉帧\n- **3D渲染**：Blender BMW 场景渲染仅需 47 秒（对比 M4 Max 58秒）\n- **编译速度**：Xcode 编译大型 Swift 项目快了约 15%\n- **续航**：正常办公 18 小时，比官方标称的 14 小时还多！\n\n### 缺点\n\n1. 价格贵（但这不是它的缺点…是我的 😅）\n2. 重量比 Air 重不少，经常外带的话选 Air\n3. 接口全在左侧，右手用鼠标时线缆会挡手\n\n**总结**：性能天花板 + 续航王者，创意工作者闭眼入。",
  "zone": "original",
  "category": "tech_digital",
  "content_type": "video",
  "tags": ["MacBook","苹果","评测","数码"]
}')
OC7_ID=$(echo "$OC7" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original MacBook评测: id=$OC7_ID"

# Original 8: 家居 - 图文
OC8=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "15㎡出租屋改造：从「城中村风」到「日系治愈小家」",
  "description": "## 改造前后对比\n\n花了不到2000块把出租屋改成了自己喜欢的模样。房东都说下次涨租要少涨一点 😂\n\n### 改造清单\n\n| 项目 | 花费 | 来源 |\n|------|------|------|\n| 米色墙纸 | ¥280 | 淘宝 |\n| 暖色落地灯 | ¥169 | 宜家 |\n| 木质置物架 ×2 | ¥320 | 拼多多 |\n| 棉麻窗帘 | ¥135 | 淘宝 |\n| 床品四件套 | ¥299 | 网易严选 |\n| 挂画/绿植 | ¥200 | 拼多多+花市 |\n| 地毯 120×180 | ¥189 | 淘宝 |\n\n### 核心思路\n\n1. **统一色调**：米色+浅木色+绿植点缀，保持视觉干净\n2. **灯光层次**：主灯+落地灯+床头灯，拒绝单一顶灯\n3. **垂直收纳**：能上墙的都上墙，省出地面空间\n4. **留白**：再小的空间也要留出「呼吸感」的角落\n\n改造是一件特别有成就感的事情。每天回到这个小窝，推开门的一瞬间——**这就是我想要的生活。**",
  "zone": "original",
  "category": "home",
  "content_type": "image",
  "tags": ["出租屋改造","家居","日系","装修"]
}')
OC8_ID=$(echo "$OC8" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 出租屋改造: id=$OC8_ID"

# Original 9: 美妆穿搭 - 图文
OC9=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "2026春夏穿搭趋势：5套「低饱和度叠穿」LOOK分享",
  "description": "## 今年春夏的几个关键趋势\n\n### LOOK 1: 燕麦色西装+垂感阔腿裤\n\n燕麦色是本季最重要的基础色，比纯白更温柔，比驼色更轻盈。搭配同色系阔腿裤，**慵懒但不邋遢。**\n\n关键单品：\n- 宽松版西装外套\n- 高腰垂感阔腿裤\n- 白色帆布鞋（故意不穿皮鞋，制造松弛感）\n\n### LOOK 2: 鼠尾草绿针织+白色百褶裙\n\n鼠尾草绿是2026春夏的流行色，低饱和度让它非常适合叠穿。外面可以再套一件米白色风衣，层次感直接拉满。\n\n### LOOK 3-5\n\n篇幅有限，剩下的三套放在了图集里。整体思路就是：**低饱和度配色 + 松紧结合 + 材质对比。**\n\n你们更喜欢哪一套？",
  "zone": "original",
  "category": "beauty_fashion",
  "content_type": "image",
  "tags": ["穿搭","春夏","OOTD","时尚"]
}')
OC9_ID=$(echo "$OC9" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 穿搭分享: id=$OC9_ID"

# Original 10: 运动 - 效率模板
OC10=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "我的「马拉松训练计划」Notion模板分享 | 从5km到全马",
  "description": "## 模板简介\n\n花了三个月打磨的马拉松训练模板，包含：\n\n1. **16周训练周期**：每周训练计划自动生成\n2. **配速计算器**：根据目标成绩自动计算各阶段配速\n3. **跑量追踪**：周跑量/月跑量自动统计+图表\n4. **营养日记**：赛前碳负荷计算器\n5. **伤病记录**：常见跑步伤病自查表\n\n### 适用人群\n\n- 跑步新手（5km起步）\n- 进阶跑者（冲击半马）\n- 严肃跑者（全马PB）\n\n### 使用说明\n\n1. 点击链接 → Duplicate to your Notion\n2. 填入你的目标比赛日期\n3. 模板会自动生成完整的16周训练日历\n4. 每天训练后打卡，配速数据导入即可分析趋势",
  "zone": "original",
  "category": "sports",
  "content_type": "template",
  "tags": ["马拉松","跑步","Notion模板","训练计划"]
}')
OC10_ID=$(echo "$OC10" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 马拉松模板: id=$OC10_ID"

# Original 11: 效率 - 纯文字
OC11=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "我用AI工具搭建了个人知识管理系统，效率提升了3倍",
  "description": "## 前言\n\n作为一个每天要处理大量信息的创作者，知识管理一直是痛点。今年尝试了各种AI工具组合后，终于找到了一套适合自己的工作流。\n\n### 工具栈\n\n- **信息收集**：Cubox（碎片化内容）+ Readwise（深度阅读）\n- **整理归档**：Obsidian（双向链接 + AI 自动标签）\n- **输出创作**：Claude（整理思路 → 扩写草稿 → 润色）\n- **项目管理**：Linear（任务追踪）+ Notion（长期规划）\n\n### 工作流\n\n```\n碎片信息 → Cubox 收集 → AI 自动摘要 → Obsidian 双向链接\n  ↓\n深度文章 → Readwise 高亮 → AI 提取要点 → Obsidian 关联旧笔记\n  ↓\n写作时 → Claude 检索 Obsidian 相关笔记 → 生成提纲 → 扩写\n```\n\n### 核心原则\n\n1. **不追求完美**：笔记不需要完整，能触发回忆就够了\n2. **AI 辅助整理，人来判断**：让AI帮你分类，但关联由你来建\n3. **定期回顾**：每周花30分钟回顾本周的笔记，建立之间的链接",
  "zone": "original",
  "category": "productivity",
  "content_type": "article",
  "tags": ["效率","知识管理","AI工具","Obsidian"]
}')
OC11_ID=$(echo "$OC11" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created original 知识管理: id=$OC11_ID"

echo ""
echo "=== Creating Fanworks ==="

# Fanwork 1: 原神同人插画
FW1=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "「雷电将军·一心净土」同人插画",
  "description": "## 创作灵感\n\n一直想画一张雷电将军在「一心净土」中的场景。这次尝试了新的光影处理——用紫色和金色对比来表现她内心的矛盾。\n\n画面中的雷电将军闭目静坐，周围的雷霆不再是武器，而是如同飘带一般环绕着她。\n\n### 绘画过程\n\n1. 线稿：2小时（面部比例调整了无数次…）\n2. 底色：1.5小时\n3. 光影：3小时（最享受的部分）\n4. 细节：2小时（发丝和雷纹）\n\n工具：iPad Pro + Procreate\n分辨率：4000×6000px\n\n希望大家喜欢！",
  "zone": "fanwork",
  "ip_id": '$IP1_ID',
  "content_type": "image",
  "tags": ["同人插画","雷电将军","一周年","电绘"]
}')
FW1_ID=$(echo "$FW1" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 雷电将军插画: id=$FW1_ID"

# Fanwork 2: 星穹铁道同人文 - 绑定原创
FW2=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "星穹同人 | 《银河列车没有终点站》",
  "description": "## 第一章：星核猎手\n\n姬子关掉了通讯器。\n\n列车的窗外是一片她从未见过的星云——紫色的气体如同波浪一般在真空中翻滚，偶尔闪过几道亮蓝色的闪电。\n\n\"还是没有信号？\"三月七靠在门框上，手里端着两杯咖啡。\n\n姬子摇了摇头。\"不只是没有信号。所有的频道都是同一个声音。\"\n\n\"什么声音？\"\n\n姬子把通讯器的外放打开。一阵低沉而有节奏的嗡鸣充满了整个驾驶室——就像一个巨大生物的心跳。\n\n\"这已经持续了三天了。\"姬子说，\"从我们进入这片星云开始。\"\n\n\"你觉得这会不会是……\"三月七的话还没说完，列车的警报系统突然响了起来。\n\n前方的紫色星云中，一个巨大的轮廓正在缓缓浮现。\n\n---\n\n*未完待续。*\n\n*本文是星穹铁道的同人创作，角色和世界观版权归米哈游所有。*",
  "zone": "fanwork",
  "ip_id": '$IP2_ID',
  "source_original_id": '$OC5_ID',
  "content_type": "article",
  "tags": ["同人文","科幻","星穹铁道","长篇"]
}')
FW2_ID=$(echo "$FW2" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 星穹同人文: id=$FW2_ID"

# Fanwork 3: 鬼灭之刃二创剪辑
FW3=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "【鬼灭之刃MAD】炭治郎之歌 × 红莲华 混剪",
  "description": "## 关于这个MAD\n\n用了整整两周时间剪出来的鬼灭MAD，把炭治郎的成长轨迹和《红莲华》的节奏对应了起来。\n\n### 剪辑思路\n\n- **0:00-0:30**：炭治郎的日常崩塌（家人遇害 ↔ 歌词\"找到了理由让自己变强\"）\n- **0:30-1:10**：训练篇（跟鳞泷修炼 ↔ 副歌爆发，节奏加快）\n- **1:10-2:00**：那田蜘蛛山篇（累之战，全篇最高潮）\n- **2:00-3:20**：无限列车篇（炎柱之死，情感收束）\n\n### 技术细节\n\n- 素材：鬼灭之刃第一季 + 无限列车剧场版 BD\n- 剪辑软件：DaVinci Resolve 18\n- 调色：模拟 ufotable 标志性的「浮世绘」色调\n- 转场：跟节拍做 cross dissolve（不是随便加特效）\n\nBGM：LiSA - 紅蓮華\n\n> 每次看炎柱那一段都会泪目。大哥永远活在我们心里。🔥",
  "zone": "fanwork",
  "ip_id": '$IP3_ID',
  "content_type": "video",
  "tags": ["MAD","鬼灭之刃","燃向","混剪"]
}')
FW3_ID=$(echo "$FW3" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 鬼灭MAD: id=$FW3_ID"

# Fanwork 4: 明日方舟同人图
FW4=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "明日方舟 | 阿米娅·升变 壁纸级插画",
  "description": "## 画给罗德岛的大家\n\n阿米娅升变形态是我在方舟里最喜欢的设计。这次挑战了4K壁纸规格。\n\n构图采用了「仰望」的视角——阿米娅站在罗德岛的甲板上，背后是移动城邦的剪影，天空中有淡淡的源石结晶光芒。\n\n她的表情介于坚定和温柔之间——就像她一直以来那样。\n\n### 画面信息\n\n- 分辨率：3840×2160（4K）\n- 格式：PNG\n- 工具：Clip Studio Paint EX\n- 耗时：16 小时\n\n提供无水印版本下载（仅个人使用）。\n\n博士，今天的罗德岛也照常运行着。",
  "zone": "fanwork",
  "ip_id": '$IP5_ID',
  "content_type": "image",
  "tags": ["阿米娅","插画","壁纸","明日方舟"]
}')
FW4_ID=$(echo "$FW4" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 阿米娅插画: id=$FW4_ID"

# Fanwork 5: 原神AI提示词
FW5=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "原神角色「胡桃」Stable Diffusion LoRA 模型分享",
  "description": "## LoRA 信息\n\n训练了一个胡桃的角色 LoRA，效果还不错，分享给大家。\n\n### 推荐参数\n\n```\nModel: Anything V5\nLoRA weight: 0.75\nSampler: DPM++ 2M Karras\nSteps: 28\nCFG: 7\nResolution: 512×768\n\nPositive Prompt:\nhu tao, (masterpiece:1.2), best quality, detailed face,\nlong dark brown hair tied in twin tails, red eyes,\nblack hat with wooden talisman, red attire with gold trim,\ngenshin impact style, soft lighting, cherry blossoms\n\nNegative Prompt:\n(worst quality:1.4), (low quality:1.4), bad anatomy,\nmissing fingers, extra digits, blurry, deformed\n```\n\n### 效果说明\n\n- 脸部还原度：★★★★☆\n- 服装还原度：★★★★★\n- 姿势泛化：★★★☆☆\n\n建议搭配 ControlNet Canny 使用，手部细节会更稳定。\n\n下载链接见附件。",
  "zone": "fanwork",
  "ip_id": '$IP1_ID',
  "content_type": "prompt",
  "tags": ["LoRA","SD","胡桃","AI绘画"]
}')
FW5_ID=$(echo "$FW5" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 胡桃LoRA: id=$FW5_ID"

# Fanwork 6: 流浪地球Mod
FW6=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "「流浪地球」KSP 行星发动机 MOD v1.2",
  "description": "## MOD 介绍\n\n在 Kerbal Space Program 中重现流浪地球的行星发动机！\n\n### 包含内容\n\n1. **行星发动机零件**（3 种规格：小型/中型/巨型）\n   - 推力：10000kN / 50000kN / 200000kN\n   - 自带蓝色能量尾焰特效\n2. **地壳锚固装置**：防止发动机把 Kerbin 推离轨道\n3. **地下城模块**：可部署的殖民地部件\n4. **领航员号空间站**：预制飞船文件\n\n### 安装方法\n\n1. 将 `WanderingEarth` 文件夹解压到 `GameData/`\n2. 确保安装了 ModuleManager 4.2+\n3. 启动游戏后按 Alt+F10 打开部件菜单\n\n### 已知问题\n\n- 巨型发动机在 VAB 中渲染时可能掉帧（KSP 引擎限制）\n- 与 Real Solar System 模组存在轻微冲突\n\n> \"道路千万条，安全第一条。行车不规范，亲人两行泪。\"",
  "zone": "fanwork",
  "ip_id": '$IP4_ID',
  "content_type": "mod",
  "tags": ["KSP","MOD","太空","流浪地球"]
}')
FW6_ID=$(echo "$FW6" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 流浪地球MOD: id=$FW6_ID"

# Fanwork 7: 鬼灭之刃同人乐谱
FW7=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "鬼灭之刃「竈門炭治郎のうた」钢琴独奏谱",
  "description": "## 扒谱说明\n\n扒了鬼灭第19集那首让人泪崩的插曲「竈門炭治郎のうた」的钢琴独奏版。\n\n### 难度\n\n- 级别：中级（英皇 5-6 级水平）\n- 调性：降E大调\n- 速度：Andante ♪=72\n- 页数：5 页\n\n### 演奏要点\n\n1. 左手琶音需要**保持均匀的力度**，不要忽快忽慢\n2. 副歌部分右手的八度跳跃需要提前准备\n3. 结尾从 pp 到 ppp 的渐弱——**最后一个和弦要像叹息一样**\n\n### 试听\n\n音频附件中包含了 MIDI 试听版本（用 Piano VST 渲染）。\n\n希望你们弹的时候不会哭。反正我扒的时候哭了。😭",
  "zone": "fanwork",
  "ip_id": '$IP3_ID',
  "content_type": "sheet_music",
  "tags": ["乐谱","钢琴","OST","鬼灭之刃"]
}')
FW7_ID=$(echo "$FW7" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 鬼灭钢琴谱: id=$FW7_ID"

# Fanwork 8: 星穹铁道二创图
FW8=$(curl -s -X POST "$BASE/contents" -H "$AUTH" -H "$CT" -d '{
  "title": "「银狼·黑客帝国」赛博朋克风格插画",
  "description": "## Cyberpunk × Silver Wolf\n\n银狼这个角色太适合赛博朋克风格了！这次尝试把她放在了类似《攻壳机动队》的世界观里。\n\n背景设定在星穹列车的下层甲板，霓虹灯管和全息广告构成了混乱但美丽的背景。银狼正在黑入一台远古终端——屏幕上滚动的是她标志性的二维码图案。\n\n### 绘画重点\n\n- 霓虹灯光在银狼头发上的反射（青色+品红）\n- 衣服材质从皮质到半透明全息材质的过渡\n- 背景中隐藏的小彩蛋：右下角有一张三月七的Wanted海报 😂\n\n希望大家能发现画面里的所有细节！",
  "zone": "fanwork",
  "ip_id": '$IP2_ID',
  "content_type": "image",
  "tags": ["赛博朋克","银狼","插画","星穹铁道"]
}')
FW8_ID=$(echo "$FW8" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "Created fanwork 银狼插画: id=$FW8_ID"

echo ""
echo "=== Seeding Complete ==="
echo "IPs: $IP1_ID, $IP2_ID, $IP3_ID, $IP4_ID, $IP5_ID"
echo "Originals: $OC1_ID, $OC2_ID, $OC3_ID, $OC4_ID, $OC5_ID, $OC6_ID, $OC7_ID, $OC8_ID, $OC9_ID, $OC10_ID, $OC11_ID"
echo "Fanworks: $FW1_ID, $FW2_ID, $FW3_ID, $FW4_ID, $FW5_ID, $FW6_ID, $FW7_ID, $FW8_ID"
