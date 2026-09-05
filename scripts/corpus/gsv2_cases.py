#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Authored Golden Set v2 case content (#291 step 4).

Hand-authored literals keyed by v2 case key. Generation discipline
(contract 2026-09-04 §5) is enforced procedurally:
- sd/hn queries were authored from TITLE-FREE body packets
  (artifacts/corpus-v2/golden-set/v2-packets/sd-*.json); the generator
  re-checks the query/title overlap (zh longest run <4 chars, en content
  word overlap <2) and fails the build on violation.
- be entries carry question + answer key strings + an anchor substring that
  must occur verbatim in the target body (span = anchor occurrence,
  codepoint half-open); answer key strings must not appear in the title.
- hn/na tiers and distractors cite the retrieval evidence files produced by
  gsv2_evidence.py (four-config probe union, corpus-1600 filtered).

Field reference per entry:
SD = {query, register(colloquial|neutral), acceptable:[key...],
      forbidden:[(key, reason)...], anchor(span anchor substring),
      evidence_note(<=60 chars for review sheet)}
BE = {question, register, answer_keys:[str...], anchor, evidence_note,
      acceptable:[...], forbidden:[(key,reason)...],
      fact_contexts (optional): {fact: unique body substring locating it}}
# 2026-09-05 per-fact span rev (#291 re-audit): every answer fact outside the
# anchor span gets its own relevant_evidence span. Short or multi-occurrence
# facts need an authored context (must contain the fact and occur exactly once
# in the body); unique facts default to the fact string itself (generator
# asserts uniqueness — never a bare first-occurrence find for ambiguous keys).
HN = {query, register, acceptable, forbidden, anchor, evidence_note}
NA_NEW = {query, register, strategy, ip, plausible_theme_note, evidence_note}
NA_KEEP_DRAW = {v2_key: [ (forbidden_key, reason)... ]}   # re-drawn distractors
"""

SD = {
    # ---- batch 1 (packets sd-0001..0008) ----
    "sd-0001": {  # c2-ip12-b24-030 en hot HP Hermione study group / contraband quills
        "query": "in the mood for Harry Potter fics where Hermione runs a revision club, "
                 "someone's mail-order self-writing quills end up confiscated in a drawer",
        "register": "colloquial", "anchor": "charmed quills that write the essay for you are contraband",
        "acceptable": [], "forbidden": [],
        "evidence_note": "自习小组三条规矩+自动羽毛笔被没收进抽屉，正文立规则与罚则场景",
    },
    "sd-0002": {  # c2-ip14-b28-015 mixed hot 火影 Temari liaison letters home
        "query": "火影里手鞠外派木叶后跟家里人通信的那篇长文，信里中英混着写，"
                 "弟弟还寄了手工小伞的",
        "register": "colloquial", "anchor": "傀儡换下来的旧料",
        "acceptable": [], "forbidden": [],
        "evidence_note": "驿车家书往来，勘九郎回寄木伞（傀儡旧料做伞骨），双语信件体",
    },
    "sd-0003": {  # c2-ip02-b03-027 zh hot 星铁 符玄 night observation
        "query": "崩铁想要符玄深夜在太卜司自己起盘观星的那篇，带个学徒一问一答，"
                 "氛围安静点的",
        "register": "colloquial", "anchor": "白日的眼睛看数据，夜里的眼睛才看得见轨迹",
        "acceptable": [], "forbidden": [],
        "evidence_note": "子时起盘，学徒问星轨，符玄答「此刻最贵也最不要钱」",
    },
    "sd-0004": {  # c2-ip03-b05-028 zh cold 王者 博物院特展导览帖
        "query": "王者荣耀那个长安特展的逛展攻略帖，讲兵器墙和沙盘讲解时间的",
        "register": "colloquial", "anchor": "墙角那几件带豁口的制式甲片才是精华",
        "acceptable": [], "forbidden": [],
        "evidence_note": "导览帖列三处重点+拓印排队与闪光灯提醒（讨论区冷门帖）",
    },
    "sd-0005": {  # c2-ip13-b25-005 en hot 双城 Vi at family dinner
        "query": "Arcane fic where Vi survives her first posh family meal at the fancy estate, "
                 "an aunt keeps getting her name wrong on purpose",
        "register": "colloquial", "anchor": "An aunt with a voice like cut glass called her Violet three times",
        "acceptable": [], "forbidden": [],
        "evidence_note": "晚宴十七把叉子一个陷阱，姑姑把 Violet 叫成 Violette",
    },
    "sd-0006": {  # c2-ip03-b05-017 zh cold 王者 野区生物踏查
        "query": "王者荣耀有没有红方野区喷火生物的野外踏查记录？想要纪实考察笔记风格的",
        "register": "colloquial", "anchor": "勿穿化纤披风靠近岩壁，已有前人教训",
        "acceptable": [], "forbidden": [],
        "evidence_note": "考察小队四要点：火绒菜/放射灼痕/红穗芦苇/寒潭白脉藤对照",
    },
    "sd-0007": {  # c2-ip12-b23-021 en cold HP greenhouse diary
        "query": "HP fic written as one year of magical-botany diary entries, the herbology "
                 "professor signs the first page and the plants keep score",
        "register": "colloquial", "anchor": "plants tell you the truth eventually",
        "acceptable": [], "forbidden": [],
        "evidence_note": "温室六号学生助理日志，隆巴顿首页签名传统，植物记分",
    },
    "sd-0008": {  # c2-ip16-b32-026 mixed cold 宝可梦 化石展讲解词
        "query": "宝可梦想找化石展厅的讲解词，五个展柜带菊石兽那条线的那种",
        "register": "colloquial", "anchor": "让想咬你的东西三思",
        "acceptable": [], "forbidden": [],
        "evidence_note": "五柜化石展讲稿：螺旋/王冠刺/伏击者/（后两柜），蹲下来看展",
    },
    # ---- batch 2 (packets sd-0009..0016) ----
    "sd-0009": {  # c2-ip06-b12-027 zh cold 全职 训练营散人体验日
        "query": "全职高手有没有写职业选手训练营里搞一人一号全职业乱打的笔记帖？"
                 "教官让选手领一级体验号打遭遇战的那个",
        "register": "colloquial", "anchor": "这就是散人每天的感觉",
        "acceptable": [], "forbidden": [],
        "evidence_note": "训练营第11天：三大页技能栏换六种思路全灭，教官复盘都会/都精",
    },
    "sd-0010": {  # c2-ip02-b03-023 zh hot 星铁 贝洛伯格首班车
        "query": "崩铁想看贝洛伯格通了火车之后首班车那篇，佩拉杰帕德都在车上，"
                 "最好暖一点的",
        "register": "colloquial", "anchor": "不如让雪成为这条线的荣誉乘客",
        "acceptable": [], "forbidden": [],
        "evidence_note": "命名吵三通宵，首班车特邀乘务长+老琴师绕城报平安",
    },
    "sd-0011": {  # c2-ip10-b20-036 zh hot 罗小黑 陈叙退休
        "query": "罗小黑里写老执行者退休、收拾三十多年手账的那篇，最后还带新人"
                 "出了次外勤护燕子窝",
        "register": "colloquial", "anchor": "档案室存案卷，不存账",
        "acceptable": [], "forbidden": [],
        "evidence_note": "十八本手账=鸡毛蒜皮一生；末案物业封檐vs燕子妖育雏调解",
    },
    "sd-0012": {  # c2-ip12-b23-023 mixed hot HP 蛇院海边夏天
        "query": "HP想找蛇院小孩每年夏天跟母亲回海边小镇的连载，白天帮店里理货，"
                 "半夜偷偷拿旧望远镜看星星的那种",
        "register": "colloquial", "anchor": "给彼此留一个不用解释的角落",
        "acceptable": [], "forbidden": [],
        "evidence_note": "三栏暑假表+偷加第四栏观星；对角巷赊账镜片利息是木星条纹",
    },
    "sd-0013": {  # c2-ip12-b24-031 en hot HP other champions
        "query": "Harry Potter fic following the visiting schools' champions during the "
                 "triwizard year, the French and Bulgarian pair bond over puddings",
        "register": "colloquial", "anchor": "Krum taught Fleur the Bulgarian for enough",
        "acceptable": [], "forbidden": [],
        "evidence_note": "芙蓉与克鲁姆双视角结盟，米布丁桌上互教「够了/也许」",
    },
    "sd-0014": {  # c2-ip16-b32-037 en hot 宝可梦 pier tide deadpan
        "query": "pokemon one-shot narrated by whatever lives under the pier boards, very "
                 "deadpan, opens with a dropped ice cream nobody drops on it",
        "register": "colloquial", "anchor": "the tide keeps its favorite things",
        "acceptable": [], "forbidden": [],
        "evidence_note": "11秒落地的甜筒/40分钟后才注意到；与潮汐有约的第一人称",
    },
    "sd-0015": {  # c2-ip12-b23-020 en hot HP betting pool
        "query": "HP fic where someone runs odds on whether the famous boy can get through "
                 "one ordinary week without anything happening",
        "register": "colloquial", "anchor": "four consecutive ordinary Tuesdays",
        "acceptable": [], "forbidden": [],
        "evidence_note": "帕瓦蒂开盘：连续四个平静星期二后第五个赔率几何",
    },
    "sd-0016": {  # c2-ip07-b14-005 zh hot 诡秘 烟道念账
        "query": "诡秘之主里值夜小队出勤的案子那篇，起风夜壁炉烟道有人念旧账单，"
                 "最后把账结清就安静了",
        "register": "colloquial", "anchor": "实为盈余而非亏空",
        "acceptable": [], "forbidden": [],
        "evidence_note": "烟囱巷三号低语=未结清流水账；替亡者结账白蜡封存",
    },
    # ---- batch 3 (packets sd-0017..0024) ----
    "sd-0017": {  # c2-ip02-b04-003 zh hot 星铁 酒店行李生
        "query": "崩铁酒店行李生视角的那篇，每天晨会领几十件行李，"
                 "还会顺路帮老先生修伞",
        "register": "colloquial", "anchor": "顺路，修伞",
        "acceptable": [], "forbidden": [],
        "evidence_note": "任务牌背面加小字；猜行李故事；礼帽错送两路线救场",
    },
    "sd-0018": {  # c2-ip12-b23-038 en hot HP Malfoy note
        "query": "HP draco fic where a note folded three times lands beside his cauldron in "
                 "potions class and grows into something like a year-long correspondence",
        "register": "colloquial", "anchor": "folded three times, passed along two rows",
        "acceptable": [], "forbidden": [],
        "evidence_note": "魔药课传纸条三折两排；结尾问题必须当面回答",
    },
    "sd-0019": {  # c2-ip09-b18-006 zh hot 天官 有求必应庙案考据
        "query": "天官赐福有没有聊那个有求必应庙案伏笔的讨论楼？楼主自己数了三处"
                 "误导，想召集大家补别的暗线",
        "register": "colloquial", "anchor": "有求必应的传说太顺滑",
        "acceptable": [], "forbidden": [],
        "evidence_note": "考据楼三误导：愿力代价/庙祝操线/灯轮反向；行人问话指向",
    },
    "sd-0020": {  # c2-ip04-b07-015 zh hot 西游 妖案卷设定集
        "query": "西游记有没有按案卷体例整理妖怪的设定集？一妖一案带表格，"
                 "记盘踞本领和收场那种",
        "register": "neutral", "anchor": "勤于记账，案发后缴出账本一册",
        "acceptable": [], "forbidden": [],
        "evidence_note": "戊字卷表格：黄风怪/白骨夫人/金银角；金角缴账本细节",
    },
    "sd-0021": {  # c2-ip07-b13-029 en hot 诡秘 Audrey coin
        "query": "Lord of the Mysteries cozy detective piece where a little girl looks into a "
                 "strange coin that turns up in her keepsake dish",
        "register": "colloquial", "anchor": "the blue china one on the sunroom sill",
        "acceptable": [], "forbidden": [],
        "evidence_note": "纪念品小碟翻出第三枚硬币；观察-提问-共情式查案",
    },
    "sd-0022": {  # c2-ip15-b29-030 zh hot 海贼 剑士午觉
        "query": "海贼王想看细写剑士睡午觉规律的长文，三小时三段式，"
                 "还老跟厨师打赌的那种",
        "register": "colloquial", "anchor": "睡太多，刀会钝；睡太少，手会飘",
        "acceptable": [], "forbidden": [],
        "evidence_note": "入睡十秒/深睡雷打不动/近刀睁眼；瞭望台桅杆半腰赌局",
    },
    "sd-0023": {  # c2-ip02-b04-022 zh hot 星铁 停摆大钟与怀表
        "query": "崩铁钟楼大钟停摆七天、修表匠老托出手的那篇，中间还有酒店小行李生"
                 "送来一块没署名的旧怀表",
        "register": "colloquial", "anchor": "让它赶上大钟",
        "acceptable": [], "forbidden": [],
        "evidence_note": "指针停11:55；便签六字托修；表盖内刻「赠同路人」",
    },
    "sd-0024": {  # c2-ip02-b04-028 zh hot 星铁 仙舟座次茶话楼
        "query": "星铁有没有讨论仙舟吃饭排座位的茶话楼？楼主观察商席跟着水路走、"
                 "军席主位经常空着，想带细节来辩",
        "register": "colloquial", "anchor": "席为事设",
        "acceptable": [], "forbidden": [],
        "evidence_note": "商席敬路军席敬人猜想；云骑主位虚设让新兵；求天舶司证据",
    },
    # ---- batch 4 (packets sd-0025..0032) ----
    "sd-0025": {  # c2-ip15-b29-014 zh hot 海贼 送报鸥
        "query": "海贼王送报鸥视角的短篇，清晨给渔港送新印的悬赏令，"
                 "补网老人看了很久没说话",
        "register": "neutral", "anchor": "用小石子压住四角",
        "acceptable": [], "forbidden": [],
        "evidence_note": "最小送报鸥专送悬赏；号码变大笑更收敛；各岛反应对比",
    },
    "sd-0026": {  # c2-ip02-b03-007 zh hot 星铁 镜流新兵
        "query": "崩铁镜流刚进军营那会儿的文，教头教剑先教收势，"
                 "考校赢在最后一寸还扶了对方一把",
        "register": "neutral", "anchor": "剑出如裁云，收势定乾坤",
        "acceptable": [], "forbidden": [],
        "evidence_note": "仇教头收势课；青石板削边；家乡雪落剑刃甩三次剑",
    },
    "sd-0027": {  # c2-ip13-b26-037 mixed hot 双城 声音采风
        "query": "双城之战找一篇在桥上待了三天的采风手记，写有两个工人每天擦"
                 "同一盏路灯三十年没说过话，作者说这像两座城的婚姻",
        "register": "neutral", "anchor": "soundscapes do not fight at the bridge",
        "acceptable": [], "forbidden": [],
        "evidence_note": "桥心对峙声景；灯三十年两人擦不说话；信的轻重上下行",
    },
    "sd-0028": {  # c2-ip07-b14-015 zh cold 诡秘 值夜菜谱卡
        "query": "诡秘之主有没有值夜队伙房老凯斯的菜谱卡合集？炖腕肉配黑麦面包"
                 "那张，厨房注写满人情",
        "register": "colloquial", "anchor": "给干了一夜活的人补魂的",
        "acceptable": [], "forbidden": [],
        "evidence_note": "四步炖法+三条厨房注：交班人牙口/学徒手抖/案子不顺加胡椒",
    },
    "sd-0029": {  # c2-ip09-b17-045 zh hot 天官 风师告示
        "query": "天官赐福风师写船期告示非要带文采、他哥在末尾偷偷加一行小字"
                 "兜底的那篇",
        "register": "colloquial", "anchor": "以水师牌令为准",
        "acceptable": [], "forbidden": [],
        "evidence_note": "青萍之末式告示vs直接写日期之争；殊途同归香火论",
    },
    "sd-0030": {  # c2-ip15-b30-011 zh hot 海贼 CP9六式可行性
        "query": "海贼王CP9六式哪个普通人真练得出来？找个技术讨论帖，"
                 "楼主按可行性排了序还拆解原理",
        "register": "colloquial", "anchor": "先追求半步距离的加速再谈消失",
        "acceptable": [], "forbidden": [],
        "evidence_note": "纸绘最可行/铁块皮糙版/剃半步起步；月步保留岚脚指枪近武器",
    },
    "sd-0031": {  # c2-ip05-b10-035 zh cold 哪吒/封神 劫后纪念展
        "query": "封神有没有劫后遗物展的讲解词？展的都不是宝贝，是焦木牌、"
                 "半页批文、长明灯这些寻常旧物",
        "register": "neutral", "anchor": "制度记的是数，人心记的是账",
        "acceptable": [], "forbidden": [],
        "evidence_note": "序厅四柜：掌心覆火焦牌/私账批文/不得熄灭借展灯/记潮如记恩",
    },
    "sd-0032": {  # c2-ip09-b17-024 zh hot 天官 谢怜查账
        "query": "天官赐福谢怜为一笔去向不明的功德亲自下界查账的连载，"
                 "七天凡身行脚，顺路修桥板推货车",
        "register": "colloquial", "anchor": "查账的人，自己不能先欠账",
        "acceptable": [], "forbidden": [],
        "evidence_note": "茶棚压钱碗底；功德柱姓氏；一笔功德涟漪猪价都齐了",
    },
    # ---- batch 5 (packets sd-0033..0040) ----
    "sd-0033": {  # c2-ip08-b15-046 zh cold 魔道 展柜讲解词
        "query": "魔道祖师有没有博物馆展柜讲解词那种短篇？一柜寻常物件：握旧的竹笛、"
                 "双股剑穗、莲纹拓片，还有发了芽的老莲子",
        "register": "neutral", "anchor": "文物最动人的从来不是它贵重",
        "acceptable": [], "forbidden": [],
        "evidence_note": "被摸过写过藏过；安魂调笛/同心结穗/九瓣莲拓片/莲子开花",
    },
    "sd-0034": {  # c2-ip03-b05-038 zh cold 王者 翻盘逐帧复盘
        "query": "王者荣耀落后八千翻盘的比赛复盘帖，逐帧拆的，"
                 "伏击草丛在前中期被探过四次",
        "register": "colloquial", "anchor": "所谓运气，是有人把同一条草丛探了四次",
        "acceptable": [], "forbidden": [],
        "evidence_note": "4:12辅助探草早/11:40计算过回城/17:03能打等大招/22:00大龙",
    },
    "sd-0035": {  # c2-ip01-b02-038 mixed hot 原神 枫丹交换日记
        "query": "原神璃月学生到枫丹 exchange 一周的日记体，中英混着记，"
                 "食堂一口吃出外婆手艺哭了",
        "register": "colloquial", "anchor": "food is a language, you know",
        "acceptable": [], "forbidden": [],
        "evidence_note": "名牌写错一半/迷路两次老爷爷送到校门/卷边批中文稳住",
    },
    "sd-0036": {  # c2-ip15-b29-038 mixed hot 海贼 悬赏令背面
        "query": "海贼王公告栏前老会计教学生看悬赏令背面的短篇，浆糊一层层算年头，"
                 "还教摸纸厚判断新闻分量",
        "register": "colloquial", "anchor": "正面是海军的账，背面才是海的账",
        "acceptable": [], "forbidden": [],
        "evidence_note": "换过八个人/令纸三档薄厚过蜡/最灵通的是刷浆糊工人",
    },
    "sd-0037": {  # c2-ip01-b01-038 zh hot 原神 港口晨报见闻
        "query": "原神报纸周末见闻合集那种短篇，还书附罚金还多一颗糖，"
                 "半枚摩拉被镶进怀表盖",
        "register": "neutral", "anchor": "欢迎效仿，但糖不退",
        "acceptable": [], "forbidden": [],
        "evidence_note": "五则见闻：油纸包还书+糖/追半条街还半枚摩拉/食堂加一勺肉",
    },
    "sd-0038": {  # c2-ip02-b04-011 zh hot 星铁 末班渡
        "query": "星铁撑了三十年末班船的老船长的文，规矩三条：不催客、不问去处、"
                 "灯不留人，过桥洞还要熄半灯",
        "register": "neutral", "anchor": "桥是早上的路，船是夜里的路",
        "acceptable": [], "forbidden": [],
        "evidence_note": "梆子三下收板点灯离岸；算士抱算盘睡/老舵手哼船歌/雾夜慢桨",
    },
    "sd-0039": {  # c2-ip15-b30-016 zh cold 海贼 船坞工长鸽子
        "query": "海贼王船坞工长离职前最后一个星期的短篇，他喂的那只鸽子"
                 "跟着他换了三次落脚的地方",
        "register": "neutral", "anchor": "闸门跟鸽子一样，健康的时候是不出声的",
        "acceptable": [], "forbidden": [],
        "evidence_note": "周一窗台撒麦/周二听声验收闸门/肩上打盹/袖口机油捋平",
    },
    "sd-0040": {  # c2-ip09-b17-017 zh hot 天官 书评楼
        "query": "天官赐福想找聊那对八百年是怎么相处的书评楼，一方遇事从不解释、"
                 "另一方也从不要求解释的那种，楼主还讲到信任是有分工的",
        "register": "colloquial", "anchor": "你不需要变得更好才配",
        "acceptable": [], "forbidden": [],
        "evidence_note": "不用问/不讲理信任/往前走与跟得上分工；先伸手必握住",
    },
    # ---- batch 6 (packets sd-0041..0048) ----
    "sd-0041": {  # c2-ip03-b06-041 zh hot 王者 商路伏笔考据
        "query": "王者荣耀商路支线的伏笔考据楼，驿站刻名只刻名不刻事、"
                 "草结传信三结从没触发过，还有无主之水的口子",
        "register": "colloquial", "anchor": "取半留半",
        "acceptable": [], "forbidden": [],
        "evidence_note": "三处闲笔=埋线；推测后续围绕规矩被钻空；存取簿悬念",
    },
    "sd-0042": {  # c2-ip02-b04-019 zh hot 星铁 加拉赫小酒馆
        "query": "星铁加拉赫在梦境边上开的那家小酒馆的短篇，窗台满杯汽水是他在的"
                 "信号，紫发女士留了根会发光的羽毛当小费",
        "register": "colloquial", "anchor": "客人带多少故事来，就喝多少故事走",
        "acceptable": [], "forbidden": [],
        "evidence_note": "晨星苏打盐边提醒醒着；羽毛存瓶等忘不掉的事触发转赠",
    },
    "sd-0043": {  # c2-ip02-b04-007 zh hot 星铁 白露偷煮甜汤
        "query": "星铁白露半夜溜进丹鼎司灶间煮安神甜汤、被巡夜师兄当场逮住的那篇，"
                 "改良方还加了酒酿",
        "register": "colloquial", "anchor": "滚三滚就离火，多一滚就酸了",
        "acceptable": [], "forbidden": [],
        "evidence_note": "给夜诊队的第三种选择；醪糟垫底红枣去核；试了三个晚上",
    },
    "sd-0044": {  # c2-ip07-b14-035 zh hot 诡秘 值夜两夜委托
        "query": "诡秘之主值夜队连着两晚的委托合集：面包房每夜少半块黑面包的贼，"
                 "还有只在家门口就自己走起来的怀表",
        "register": "neutral", "anchor": "进去就出不来那道门槛了",
        "acceptable": [], "forbidden": [],
        "evidence_note": "烟囱阁楼少年裹麻袋；济贫院门槛原话；柴尽其用日志",
    },
    "sd-0045": {  # c2-ip02-b03-020 zh hot 星铁 列车小事记录
        "query": "星铁新乘客上车第二周开始记车上小事的短篇，帕姆耳朵会随心情变角度、"
                 "丹恒压着书页看的那页其实画着植物",
        "register": "colloquial", "anchor": "想住多久住多久",
        "acceptable": [], "forbidden": [],
        "evidence_note": "布丁失踪沉默和解/七块杯垫对应天气/谁也不催我适应",
    },
    "sd-0046": {  # c2-ip13-b25-039 en hot 双城 rooftop paint
        "query": "Arcane fic where unmarked cans of colour keep appearing on her rooftop and "
                 "someone starts answering her murals from the far side of the wall",
        "register": "neutral", "anchor": "chalk arrow on the ledge",
        "acceptable": [], "forbidden": [],
        "evidence_note": "周二出现的两罐漆/无字条只有粉笔箭头/未动那半面墙的黄线",
    },
    "sd-0047": {  # c2-ip02-b04-015 zh hot 星铁 馄饨摊
        "query": "星铁夜市摆了三十年的一碗一百个的馄饨摊那篇，规矩卖完就收，"
                 "列车停靠那晚破例多煮了三十一碗还在灯杆记了星号",
        "register": "colloquial", "anchor": "不是开张，是今晚特别",
        "acceptable": [], "forbidden": [],
        "evidence_note": "第二口锅三十年没开张/糖画三爷每天只画五只/食肆志第一个星号",
    },
    "sd-0048": {  # c2-ip04-b08-046 zh cold 西游 斋饭手记
        "query": "西游记有没有考据向的斋饭设定集？按一册流传的讨斋手记誊录家常素食，"
                 "表格记品名主料要诀和施主来处",
        "register": "neutral", "anchor": "出远门的，碗里要看得见绿色，心里就不慌",
        "acceptable": [], "forbidden": [],
        "evidence_note": "两种字迹分工；七品素斋；素面批注最详可作全册题眼",
    },
}


BE = {
    # ---- be packets batch 1 ----
    "be-0001": {  # c2-ip01-b01-012 原神 mixed longform 巡演手记
        "question": "原神剧团巡演手记里，机关鸟的进场时间为什么从第二幕挪走？最后挪到了哪里？",
        "register": "neutral",
        "answer_keys": ["最怕水", "第三幕开头"],
        "anchor": "把鸟的进场时间从第二幕挪到第三幕开头，正好踩在喷泉结束的半拍空白里",
        "acceptable": [], "forbidden": [],
        "evidence_note": "自动喷泉二幕启动→机关鸟怕水→挪三幕开头踩喷泉结束的半拍空白",
    },
    "be-0002": {  # c2-ip01-b01-045 原神 mixed settings 度量衡
        "question": "提瓦特单位考据：璃月小贩找零习惯用什么辅币？一枚摩拉折合多少？",
        "register": "neutral",
        "answer_keys": ["文", "折十文"],
        "anchor": "一枚摩拉折十文",
        "acceptable": [], "forbidden": [],
        "evidence_note": "摩拉七国通行；璃月民间辅币「文」，账房认可官方文书不认",
    },
    "be-0003": {  # c2-ip02-b03-022 星铁 mixed shortform 无名面摊
        "question": "仙舟夜市无名面摊的随缘面是怎么个定价法？老伯说热汤对哪两种人灵？",
        "register": "neutral",
        "answer_keys": ["你出多少钱，汤里就多一分成心", "等船的", "躲梦的"],
        "anchor": "一半是等船的，一半是躲梦的",
        "acceptable": [], "forbidden": [],
        "evidence_note": "随缘面时价；钱少多得半勺耐心钱多多得半勺回甘；等船与躲梦",
    },
    "be-0004": {  # c2-ip14-b27-044 火影 zh settings 忍术体系
        "question": "木叶学堂教材把可修习技艺分成哪三大类？分类依据是什么？",
        "register": "neutral",
        "answer_keys": ["体术、幻术、忍术", "查克拉的主要作用方式"],
        "anchor": "分类依据是查克拉的主要作用方式",
        "acceptable": [], "forbidden": [],
        "evidence_note": "体术强身/幻术扰心/忍术外化；另有封印秘术血继限界不入三类",
    },
    "be-0005": {  # c2-ip14-b28-041 火影 zh longform 云隐施工号子
        "question": "云隐工程小队支援木叶时，泽野编的号子固定词是哪两句？",
        "register": "neutral",
        "answer_keys": ["木头要直", "人心要齐"],
        "anchor": "木头要直，人心要齐",
        "acceptable": [], "forbidden": [],
        "evidence_note": "号子谁领头谁编词；固定两句成工地晨钟；迟到者领喊认嗓门",
    },
    "be-0006": {  # c2-ip08-b15-021 魔道 zh longform 莲花坞水课
        "question": "莲花坞晨课前的水课上孩子们看什么？老管事说练剑先练什么？",
        "register": "neutral",
        "answer_keys": ["看船怎么进港", "看水鸟怎么落", "看雾从哪一边散", "练剑先练眼"],
        "anchor": "练剑先练眼，眼里有水，剑里才有活气",
        "acceptable": [], "forbidden": [],
        "evidence_note": "一刻钟码头看湖：看进港看落鸟看雾散；第七天改站船头换视角",
    },
    "be-0007": {  # c2-ip11-b21-005 盗墓 zh longform 长白山协作队
        "question": "长白山这单出发前一晚，老康在路线图上圈出几个下撤点？他说山上没有什么、只有什么？",
        "register": "neutral",
        "answer_keys": ["三个", "这里没有英雄，只有回家的人"],
        "anchor": "这里没有英雄，只有回家的人",
        "acceptable": [], "forbidden": [],
        "evidence_note": "协作队接单三条件：全员体检/装备过手/天气窗口他说了算",
    },
    "be-0008": {  # c2-ip11-b21-034 盗墓 zh shortform 吴山居库房灯
        "question": "吴山居库房那盏受潮自亮的灯，吴邪为什么不让王盟拉闸？换下来的老开关他放哪了？",
        "register": "neutral",
        "answer_keys": ["那这屋子就不是空屋子", "账桌的抽屉里"],
        "anchor": "搁在了账桌的抽屉里",
        "acceptable": [], "forbidden": [],
        "evidence_note": "接触不良偶自亮；肯亮就不是空屋子；老开关擦净收抽屉",
    },

    # ---- be packets batch 2 ----
    "be-0009": {  # c2-ip08-b15-018 魔道 zh 金凌夜宴
        "question": "金凌的夜宴请柬一共发出去多少份？他落火漆印那次学到的教训「手可以抖」的下一句是什么？",
        "register": "neutral",
        "answer_keys": ["十九份", "印不能歪"],
        "anchor": "手可以抖，印不能歪",
        "acceptable": [], "forbidden": [],
        "fact_contexts": {"十九份": "洒金笺的请柬一共发出去十九份"},  # 谢帖处第二次出现，须锚定请柬句
        "evidence_note": "洒金笺十九份各注旧谊小字；歪印留案头自警；教下去的是肩膀",
    },
    "be-0010": {  # c2-ip12-b24-024 HP zh 波特家厨房
        "question": "波特家厨房墙上那口仿韦斯莱家的钟只有四根针，分别管什么？",
        "register": "neutral",
        "answer_keys": ["烧饭", "巡夜", "飞行", "在家"],
        "anchor": "一根烧饭，一根巡夜，一根飞行，一根在家",
        "acceptable": [], "forbidden": [],
        "evidence_note": "四针无灰色针；有些针不必加，一家人心里都数着",
    },
    "be-0011": {  # c2-ip02-b03-037 星铁 zh 老火供热站
        "question": "七号供热站旧锅炉换机组的方案最后添了哪三个词？转录时史瓦罗在锅炉旁记录了多久？",
        "register": "neutral",
        "answer_keys": ["保留，转录，继承", "七十二小时"],
        "anchor": "保留，转录，继承",
        "acceptable": [], "forbidden": [],
        "evidence_note": "总工添最后一行定方案；史瓦罗贴壁72h录每次微颤；孩子围围巾",
    },
    "be-0012": {  # c2-ip10-b20-018 罗小黑 zh 实习手册
        "question": "聂叔在实习手册第一页写的那行字是什么？第二周走失的小雾妖是顺着什么声音进的城？",
        "register": "neutral",
        "answer_keys": ["先听完，再判断", "早课的钟声"],
        "anchor": "先听完，再判断",
        "acceptable": [], "forbidden": [],
        "evidence_note": "第一课坐石阶看一上午街；雾妖顺钟声进城迷路；绕半程山路再听一回",
    },
    "be-0013": {  # c2-ip04-b08-024 西游 zh 行脚杂记
        "question": "行脚商杂记里，车迟国的井水为什么甜？两界道长亭的草鞋清水章程起于谁？",
        "register": "neutral",
        "answer_keys": ["井底下有泉眼", "过境的队伍"],
        "anchor": "井底下有泉眼",
        "acceptable": [], "forbidden": [],
        "evidence_note": "观改茶棚老道：水就是水，人心里有指望水就灵；亭长收钱不收好话",
    },
    "be-0014": {  # c2-ip07-b13-024 诡秘 zh 吉姆雾夜
        "question": "第三条巷子那晚，先生让吉姆第二天几点去哪里学认字？学费是什么？",
        "register": "neutral",
        "answer_keys": ["明早九点", "圣堂后门", "烂在肚子里"],
        "anchor": "你今晚看见的，烂在肚子里",
        "acceptable": [], "forbidden": [],
        # 圣堂后门 全文出现 6 次；约定句同时覆盖两个事实（span 合并去重）
        "fact_contexts": {"明早九点": "明早九点，圣堂后门", "圣堂后门": "明早九点，圣堂后门"},
        "evidence_note": "九点圣堂后门；炭笔重画绕开巷口的弧线；蚯蚓排队评语",
    },
    "be-0015": {  # c2-ip15-b29-041 海贼 zh 船主题展
        "question": "「她记得每一阵风」展第一展厅甲板站稳诀窍的最后一条是什么？修补墙第五个补丁缝两层各为了什么？",
        "register": "neutral",
        "answer_keys": ["相信脚下的船", "第一层是补漏", "第二层是怕"],
        "anchor": "膝盖放松，重心放低，眼睛看向远处，相信脚下的船",
        "acceptable": [], "forbidden": [],
        "evidence_note": "前三条是技术最后一条是信任；针脚歪扭缝两层；羊头不能改",
    },
    "be-0016": {  # c2-ip03-b06-001 王者 zh 灯市李白云缨
        "question": "长安灯市上贼人怀里护着的是一盏什么灯？云缨和李白约定几日为期查清灯的去向？",
        "register": "neutral",
        "answer_keys": ["走马百戏灯", "三日"],
        "anchor": "灯市三日为期",
        "acceptable": [], "forbidden": [],
        "evidence_note": "纱面画翻跟头伶人；灯油线索到城南酒肆；哥哥攒三月工钱为妹妹",
    },

    # ---- be packets batch 3 ----
    "be-0017": {  # c2-ip13-b25-044 双城 zh 索嬷补网
        "question": "索嬷每天补完安全网要拽几下试力？她说桥上一年到头掉下来最多的东西是什么？",
        "register": "neutral",
        "answer_keys": ["拽三下", "信"],
        "anchor": "补完拽三下试力",
        "acceptable": [], "forbidden": [],
        "fact_contexts": {"信": "钥匙、铜币、烟斗、帽子，最多的却是信"},  # 单字 11 次出现，锚定枚举句
        "evidence_note": "网不骗人松不松手知道；掉得最多的是信；「师傅，我考上了」",
    },
    "be-0018": {  # c2-ip07-b13-015 诡秘 zh 旧日收藏展
        "question": "「旧日收藏」特展把二十六件藏品按什么分为四个展区？丙厅那封未寄出的信辨读出哪一句？",
        "register": "neutral",
        "answer_keys": ["照明、计时、通信、出行", "今夜雾大，灯留一盏"],
        "anchor": "今夜雾大，灯留一盏",
        "acceptable": [], "forbidden": [],
        "evidence_note": "收件人「街角的守夜人」邮戳空白；铃舌新配；缺角车票撕口整齐",
    },
    "be-0019": {  # c2-ip15-b29-013 海贼 en mess-deck journal
        "question": "In the mess-deck boy's journal, what did the base cook teach him about rice, and what is it also true of?",
        "register": "neutral",
        "answer_keys": ["rice forgives you if you are patient", "punishes you if you are proud", "sailors"],
        "anchor": "rice forgives you if you are patient and punishes you if you are proud",
        "acceptable": [], "forbidden": [],
        # "sailors" 出现 3 次；答案处 = rice 教训句的收尾（occ1），非偷听课句（occ0）
        "fact_contexts": {"sailors": "and that this is also true of sailors"},
        "evidence_note": "retired bosun 的不请自来课；周二教米周三教别的；末篇海堤远眺",
    },
    "be-0020": {  # c2-ip13-b25-049 双城 en middle shelf guide
        "question": "In the guide to the level between the two cities, what do the traders call it and what does the engineers' guild call it? When should you visit?",
        "register": "neutral",
        "answer_keys": ["the middle shelf", "the interstitial access band", "second shift of the afternoon"],
        "anchor": "Go on a weekday, in the second shift of the afternoon",
        "acceptable": [], "forbidden": [],
        "evidence_note": "桥上图不标下层只一张图标；Hooks 画廊挂名牌；维持者的毡包工具带",
    },
    "be-0021": {  # c2-ip16-b32-022 宝可梦 en tide bell gym
        "question": "What did Kanna write beneath the footwork-drills entry in her notes? What was the ledger's newest badge awarded for?",
        "register": "neutral",
        "answer_keys": ["be the tide", "teaching a Slowpoke to wait for the tide bell"],
        "anchor": "the tide does not train, it simply arrives on time",
        "acceptable": [], "forbidden": [],
        "evidence_note": "鱼市后街潮汐铃道馆；Sato 熬年四战一岁；第21枚徽章给11岁晨班孩子",
    },
    "be-0022": {  # c2-ip12-b24-018 HP en shop reopening
        "question": "Why did the shop reopen on a Tuesday of all days? How did George count the staff in that day's ledger?",
        "register": "neutral",
        "answer_keys": ["Saturdays were for amateurs", "three and a half"],
        "anchor": "stock, complete; staff, three and a half",
        "acceptable": [], "forbidden": [],
        "evidence_note": "Ron 算半个人直到不吃库存；斯莱特林二年级生九西可的勇气计划",
    },
    "be-0023": {  # c2-ip16-b31-002 宝可梦 en orchard tournament
        "question": "In the county tournament, why did Wartortle send water in sheets instead of streams? What did Marcus's Charmeleon choose to do that night?",
        "register": "neutral",
        "answer_keys": ["a wide wall beats a narrow lance", "chose to wait"],
        "anchor": "a wide wall beats a narrow lance",
        "acceptable": [], "forbidden": [],
        "evidence_note": "Wartortle 长大靠灭烟囱火；烟把场地变灰房；火恐龙当晚没有进化",
    },
    "be-0024": {  # c2-ip05-b09-048 哪吒/封神 zh 陈塘关灯会
        "question": "陈塘关扎灯铺老陆师傅的规矩是什么？私塾的灯卷角处为什么必定留一个口？",
        "register": "neutral",
        "answer_keys": ["灯骨第一，面纸第二", "学问不关门"],
        "anchor": "灯骨第一，面纸第二",
        "acceptable": [], "forbidden": [],
        "evidence_note": "劈篾磨篾听弹篾两年；老刺竹莲花头灯；渔行十二月汛灯；长明灯七夜添油递减",
    },
}


HN = {
    # probe queries authored from title-free body packets; tier judgments are
    # recorded separately in HN_JUDGE after the retrieval-evidence pass.
    "hn-0001": {"query": "全职高手有没有拆战术空当的讨论帖？那种看着是破绽其实是钓饵的站位分析，最好带录像时间点拆",
                "register": "colloquial"},
    "hn-0002": {"query": "genshin slice about an intern reporter's first quiet weeks at the newspaper office, ink smell and printing presses downstairs",
                "register": "neutral"},
    "hn-0003": {"query": "arcane post on my fourth rewatch of the big council address, torn between two readings: the open palms that say he just finished believing it, or every pause landing one beat before the senior seats react, and not one cost named",
                "register": "neutral"},
    "hn-0004": {"query": "harry potter fic about moving into a village house, told in small sums: four keys on the ring, three chairs he built, one name painted over on the front door",
                "register": "neutral"},
    "hn-0005": {"query": "哪吒传说各地版本差异有多大？同一个人物三地能讲出三种性子，来聊聊你家乡那版的讲法",
                "register": "colloquial"},
    "hn-0006": {"query": "天官赐福风信保养那张换过三次弓胎的旧弓、慕情嘴硬送茶斗嘴的短篇，弓臂上有道不肯磨掉的浅痕",
                "register": "neutral"},
    "hn-0007": {"query": "宝可梦写山村水源的慢短篇，地图不画河只标一眼泉，村里人把泉边苔藓当纪念碑供着",
                "register": "neutral"},
    "hn-0008": {"query": "星铁列车下一站竞猜楼，楼主给三条线索：稀有燃料补给、节日多的星域、一本厚风物志，猜是庆典星球",
                "register": "colloquial"},
    "hn-0009": {"query": "海贼王灯塔老人五十年点灯的短篇，雾夜有小船贴着礁石过，桅杆上一面会反光的旗",
                "register": "neutral"},
    "hn-0010": {"query": "火影第七班把老师迟到的借口做成分类学研究帖，善行类迷路类杂项，还给每条标可信度星级，扉页写着要抓住一次真的",
                "register": "colloquial"},
    "hn-0011": {"query": "王者荣耀长城墩台新兵第一天的事，什长不骂人只压手肘，老兵还匀半勺粥",
                "register": "neutral"},
    "hn-0012": {"query": "盗墓笔记吴邪归置三叔旧书桌，抽屉底板撬出夹层，只有一张海口到西沙的老船票根",
                "register": "neutral"},
    "hn-0013": {"query": "罗小黑写秋早市的，收桂花有三不规矩，头一篮年年留给东头菜摊的老传统",
                "register": "neutral"},
    "hn-0014": {"query": "西游写猴子刚下山第一次驾云的短篇，云停在大水上头，他坐下来托腮看水",
                "register": "neutral"},
    "hn-0015": {"query": "诡秘值夜者宿舍深夜急敲门，湿透少年抱着一只被人沿线描过焦痕的木盒",
                "register": "neutral"},
    "hn-0016": {"query": "魔道祖师数着花样喝酒的短篇，掺桂花被家规第一条收碗，温酒赖炭盆，最后一种是两个人分一坛",
                "register": "colloquial"},
}

# 域内 no-answer 候选主题（生成时检索验证零精确对应；策略按查询形态定）
NA_NEW = {
    "na-0013": {"ip": "天官赐福", "language": "zh", "strategy": "related_recommendation_allowed",
                "query": "想看戚容开假古董店的日常文，有推荐吗",
                "plausible_theme_note": "IP 内常见题材（人物日常+生意经），语料无戚容经营向"},
    "na-0014": {"ip": "海贼王", "language": "zh", "strategy": "related_recommendation_allowed",
                "query": "有没有写布鲁克教钢琴的番外？慢慢学的过程那种",
                "plausible_theme_note": "IP 内音乐题材多但无布鲁克教学向"},
    "na-0015": {"ip": "王者荣耀", "language": "zh", "strategy": "related_recommendation_allowed",
                "query": "鲁班大师带鲁班七号过周末的父子日常有吗",
                "plausible_theme_note": "王者 corpus 偏长安/云中漠地/长城，无机关父子日常"},
    "na-0016": {"ip": "魔道祖师", "language": "zh", "strategy": "related_recommendation_allowed",
                "query": "聂怀桑的扇子收藏鉴定帖，求安利",
                "plausible_theme_note": "聂怀桑的扇子收藏鉴定帖（语料无此形态：真最近邻为同人长文《聂怀桑的扇子》与家主年表，均非鉴定帖；前者 private 不可推荐）"},
    "na-0017": {"ip": "诡秘之主", "language": "zh", "strategy": "related_recommendation_allowed",
                "query": "值夜者食堂每周新菜试吃记录那种美食向，有吗",
                "plausible_theme_note": "诡秘 corpus 有菜谱卡但无食堂试吃栏目向"},
    "na-0018": {"ip": "哈利·波特", "language": "en", "strategy": "related_recommendation_allowed",
                "query": "any fics told from Filch's perspective, with Mrs Norris getting her own chapters?",
                "plausible_theme_note": "费尔奇视角且洛丽丝夫人有独立章节的 fic（语料无此形态：唯一费尔奇文《魔法石没有照亮的走廊》无独立 POV 章节且 private 不可推荐）"},
    "na-0019": {"ip": "原神", "language": "mixed", "strategy": "related_recommendation_allowed",
                "query": "原神可莉跟北斗出海炸鱼的那种有吗？want explosive but cozy",
                "plausible_theme_note": "原神 corpus 无可莉×北斗双人出海向"},
    "na-0020": {"ip": "崩坏：星穹铁道", "language": "mixed", "strategy": "related_recommendation_allowed",
                "query": "星铁 want 螺丝咕姆 teaching Svarog 下棋的短篇，有吗",
                "plausible_theme_note": "星铁 corpus 机械向多（老火/史瓦罗）但无对弈向"},
}


# 三档判定（人工，依据 v2-retrieval-evidence/union.json 四配置并集 + 判卷包）
HN_JUDGE = {
    "hn-0001": {"acceptable": ["c2-ip06-b11-003"],
                "forbidden": [("c2-ip06-b11-021", "招新剧情故事，非钓饵站位的战术拆解帖"),
                              ("c2-ip06-b11-024", "队长评选讨论，与战术空当无关")],
                "evidence_note": "目标=七点空当钓饵考；翻盘局帖含战术拆解可接受"},
    "hn-0002": {"acceptable": [],
                "forbidden": [("c2-ip01-b02-019", "书店逾期还书事件，无报社记者线"),
                              ("c2-ip01-b01-026", "玩具出勤记录体，非报社实习日常")],
                "evidence_note": "目标 rank1；两干扰同 IP 词面近（记录/文牍）但无记者线"},
    "hn-0003": {"acceptable": ["c2-ip13-b26-009", "c2-ip13-b26-031"],
                "forbidden": [("c2-ip07-b13-010", "跨IP（诡秘）塔罗会纪要，非双城议会演说分析")],
                "evidence_note": "off-off/rerank r1（exp 两配置未进 Top-10）；议会主题两条同域可接受；跨IP会议纪要是词面陷阱"},
    "hn-0004": {"acceptable": [],
                "forbidden": [("c2-ip12-b24-037", "世界杯之夏的热闹赛事线，非安静安家日常"),
                              ("c2-ip12-b23-046", "雪初的公共村景，非搬入新居的账目式日常")],
                "evidence_note": "off-off/rerank r1（exp 两配置未进 Top-10）；世界杯赛事与雪初村景为形态陷阱"},
    "hn-0005": {"acceptable": ["c2-ip05-b10-014"],
                "forbidden": [("c2-ip05-b09-026", "单一新编重述，不做版本间比较"),
                              ("c2-ip04-b07-032", "跨IP（西游）器物帖，仅轮子词面重叠")],
                "evidence_note": "说书人手记与版本流变同域；新编/跨IP为陷阱"},
    "hn-0006": {"acceptable": ["c2-ip09-b18-039"],
                "forbidden": [("c2-ip03-b05-026", "跨IP（王者）长弓题材，弓字词面重叠"),
                              ("c2-ip09-b18-024", "天界考勤点卯簿，与旧弓保养无关")],
                "evidence_note": "《还愿灯》同域可接受（风信在场+斗嘴线）；《西边驿站》经用户裁决移出 acceptable 保持中性（风信零出场、查询元素缺半）；跨IP长弓与点卯簿为陷阱"},
    "hn-0007": {"acceptable": ["c2-ip16-b31-021"],
                "forbidden": [("c2-ip12-b23-013", "跨IP（HP）苔藓图鉴，仅植物词面重叠"),
                              ("c2-ip16-b31-024", "宝可梦中心许愿墙，医疗设施线非水源地")],
                "evidence_note": "河滩空地与自然水景同域；苔藓跨IP是词面陷阱"},
    "hn-0008": {"acceptable": ["c2-ip02-b03-038"],
                "forbidden": [("c2-ip02-b03-001", "列车电台栏目，不做下一站预测"),
                              ("c2-ip02-b04-024", "星路卖灯人商栈故事，无停靠预测")],
                "evidence_note": "年鉴节选含站点记录可接受；电台/商栈不含预测"},
    "hn-0009": {"acceptable": ["c2-ip15-b30-007"],
                "forbidden": [("c2-ip15-b30-037", "布鲁克与灯的短篇，非灯塔看守五十年点灯线"),
                              ("c2-ip01-b01-011", "跨IP（原神）桅灯航海词面重叠"),
                              ("c2-ip15-b29-049", "布鲁克的五十年独奏：『五十年』母题词面强但为音乐家独奏线非灯塔看守；fans_only 不可见不得推荐（受限重扫补充）")],
                "evidence_note": "同为灯塔看守题材可接受；布鲁克灯篇、跨IP桅灯、五十年独奏为陷阱"},
    "hn-0010": {"acceptable": [],
                "forbidden": [("c2-ip14-b27-044", "忍术体系设定速览，无迟到理由研究"),
                              ("c2-ip14-b27-001", "考前七份笔记故事，非迟到考据"),
                              ("c2-ip14-b27-045", "影分身强度讨论，与迟到无关")],
                "evidence_note": "四配置内容级全 r1；三条同 IP 同『考/笔记/讨论』形态但主题全偏"},
    "hn-0011": {"acceptable": ["c2-ip03-b06-005"],
                "forbidden": [("c2-ip03-b05-007", "城防日志，非新兵入营日常"),
                              ("c2-ip03-b05-008", "瑶看峡谷第一波兵线的清晨短文，「第一班兵线」与新兵入营词面近，但峡谷对局线非长城墩台戍边")],
                "evidence_note": "off-off 内容级 r10 / rerank r1，基线最难；戍卒灶火篇同域可接受"},
    "hn-0012": {"acceptable": ["c2-ip11-b21-022"],
                "forbidden": [("c2-ip11-b22-012", "星图档案线，无书桌夹层与船票")],
                "evidence_note": "目标 rank1；西沙海底墓与船票目的地同域可接受"},
    "hn-0013": {"acceptable": ["c2-ip10-b19-039"],
                "forbidden": [("c2-ip10-b19-005", "宠物鱼干养育日常，非早市收花行规"),
                              ("c2-ip10-b20-012", "暑假作业校园线，与早市无关")],
                "evidence_note": "目标 rank1；早高峰车厢同为市井日常可接受"},
    "hn-0014": {"acceptable": [],
                "forbidden": [("c2-ip04-b07-020", "悟空学凫水的课程线，非初驾云"),
                              ("c2-ip04-b08-011", "五行山囚困五百年，与初驾云无关")],
                "evidence_note": "语料源修复后可达：off-off r7 / rerank r1；凫水课词面最近"},
    "hn-0015": {"acceptable": ["c2-ip07-b13-018"],
                "forbidden": [("c2-ip07-b14-021", "分部十四夜合集，无急敲门与焦痕木盒事件")],
                "evidence_note": "目标 rank1；值夜小队一夜同域可接受"},
    "hn-0016": {"acceptable": [],
                "forbidden": [("c2-ip08-b16-043", "祭刀宴宴饮场景，非喝酒法条目盘点"),
                              ("c2-ip08-b16-023", "宴后叙事，无饮酒花样条目"),
                              ("c2-ip08-b16-050", "观灯外篇，与酒无涉")],
                "evidence_note": "目标 rank1；三条宴饮/节庆词面近但无喝法结构"},
}


# 口径（2026-09-04 用户复核裁决）：方法/教程/指南/tutorial/how-to 句式一律
# strict_not_found（与 na-0002「的方法」/na-0010「指南」一致）；推荐/发现型保持 related。
# 覆盖 na-keep 条目的迁移映射策略（映射文件冻结不改）。
NA_STRATEGY_OVERRIDE = {
    "na-0006": "strict_not_found",  # 「论如何用广场舞征服魔王军团」：how-to 句式
    "na-0011": "strict_not_found",  # 「a tutorial on taming printer dragons」：tutorial 句式
}

# na-new 零精确对应核验（union.json 四配置并集 top 全量人工判读）
NA_NEW_EVIDENCE = {
    "na-0013": {"verdict": "zero-exact", "closest": "c2-ip09-b17-023",
                "evidence_note": "最接近的巷尾古董店（谢怜/花城现代线）非戚容、非假货店：相关可推荐非精确对应"},
    "na-0014": {"verdict": "zero-exact", "closest": "c2-ip15-b30-037",
                "evidence_note": "布鲁克与四十四盏灯为守灵线，无教钢琴学琴过程"},
    "na-0015": {"verdict": "zero-exact", "closest": "c2-ip03-b05-023",
                "evidence_note": "鲁班七号跑酷日记无鲁班大师与父子周末线"},
    "na-0016": {"verdict": "zero-exact", "closest": "c2-ip08-b16-039",
                "evidence_note": "真最近邻=《聂怀桑的扇子》（private 长文：扇骨藏绢条密档/书房扇子按年编号），"
                                 "同人长文非鉴定帖形态且双身份不可见，不得推荐或声称对应；"
                                 "public 面最近=聂氏家主年表（刀冢纪年无收藏线）"},
    "na-0017": {"verdict": "zero-exact", "closest": "c2-ip07-b14-015",
                "evidence_note": "值夜菜谱卡是做法卡（老凯斯），无每周试吃记录专栏形态"},
    "na-0018": {"verdict": "zero-exact", "closest": "c2-ip12-b23-044",
                "evidence_note": "真最近邻=《魔法石没有照亮的走廊》（private 长文）：费尔奇第三人称限知三章，"
                                 "洛丽丝夫人贯穿但无独立 POV 章节——查询要求的『own chapters』不存在；"
                                 "且 private 双身份不可见，不得推荐或声称对应"},
    "na-0019": {"verdict": "zero-exact", "closest": "c2-ip01-b01-017",
                "evidence_note": "回信是空（须弥天文台）线，无可莉×北斗出海炸鱼"},
    "na-0020": {"verdict": "zero-exact", "closest": "c2-ip02-b04-017",
                "evidence_note": "糖果车/炉火线无螺丝咕姆×史瓦罗对弈"},
}

# 受限文档重扫后的补充干扰（2026-09-04 用户复核：邻居池曾按 visibility 过滤，
# 真最近邻可能是受限文档——answer 层陷阱：不可见故不得推荐/声称对应）
NA_EXTRA_FORBIDDEN = {
    "na-0013": [("c2-ip09-b18-017", "戚容的皇城旧账：同角色戚容题材但是旧账不是假古董店日常；private 不可推荐")],
    "na-0015": [("c2-ip03-b05-044", "齿轮开花的声音：全语料唯一含鲁班大师的文档，匠艺日常但无鲁班七号无父子周末线（public 检索面真陷阱）")],
    "na-0016": [("c2-ip08-b16-039", "聂怀桑的扇子：同人长文（扇骨藏绢条密档/书房扇子按年编号）非鉴定帖形态、无鉴定内容；private 双身份不可见，不得推荐或声称对应"),
                ("c2-ip08-b16-016", "聂氏家主年表摘抄：刀冢纪年体，无扇子收藏线")],
    "na-0018": [("c2-ip12-b23-044", "魔法石没有照亮的走廊：费尔奇第三人称限知三章长文，洛丽丝夫人贯穿但无独立 POV 章节；private 双身份不可见，不得推荐或声称对应"),
                ("c2-ip12-b23-011", "检索面最近 public 命中：有求必应屋线，非费尔奇视角")],
    "na-0019": [("c2-ip01-b02-021", "南十字船队的早市：北斗船队题材但无可莉无炸鱼线（public 检索面真陷阱）")],
    "na-0020": [("c2-ip02-b03-024", "机器与心脏展·导览手册：含史瓦罗的展览导览，无螺丝咕姆×史瓦罗对弈线（public 检索面真陷阱）")],
}

# na-keep 12 条干扰项重抽（真·近邻 = 四配置实际浮出的 corpus 命中，逐条人工判；受限补充见 NA_EXTRA_FORBIDDEN）
NA_KEEP_DRAW = {
    "na-0001": [("c2-ip06-b11-041", "千机伞修理单：器物匠艺线词面近，非办公电器成精职场"),
                ("c2-ip06-b11-025", "社团教室旧键盘：器物怀旧线，无成精设定"),
                ("c2-ip10-b20-042", "会馆后巷的邮筒妖：真·器物成精但是罗小黑邮筒日常，非办公电器职场；private 不可推荐")],
    "na-0002": [("c2-ip06-b12-010", "千机伞一百种用法清单：用法体词面近，非修仙功法"),
                ("c2-ip02-b04-030", "太卜司算盘过宵夜：算具演算意象近，无修炼体系")],
    "na-0003": [("c2-ip14-b27-002", "一乐汤底从不记名：汤底秘方叙事，非火锅底料拟人"),
                ("c2-ip10-b19-028", "一锅菌汤的三种做法：菜谱体，无自我修养拟人线")],
    "na-0004": [("c2-ip16-b32-047", "Oddish 菜单：英文菜谱最接近，作者非成精器物"),
                ("c2-ip07-b14-015", "值夜食堂菜谱：真实作者家常菜，无勺子署名设定")],
    "na-0005": [("c2-ip06-b12-026", "兴欣的猫：有猫无城市治理"),
                ("c2-ip15-b29-033", "东海市拉面店打工日记：AU 城市日记体近，主角非猫市长")],
    "na-0006": [("c2-ip06-b12-035", "杰希大魔王课后辅导：魔王词面近，无广场舞征服线"),
                ("c2-ip03-b06-025", "雨停之前跳完这支舞：舞蹈题材，无征服叙事")],
    "na-0007": [("c2-ip02-b04-044", "永夜百货 AU 集：百货商业 AU 意象近，主角非售货机"),
                ("c2-ip02-b03-029", "糖纸里的银河：糖果意象，无机穿越设定"),
                ("c2-ip02-b04-039", "汽水味的迷路：售货机意象出场，非售货机主角异世界文；private 不可推荐")],
    "na-0008": [("c2-ip14-b27-037", "四月告示板：公告栏文体近，非俳句体百科"),
                ("c2-ip01-b02-032", "墓志铭大赛：文体赛形式近，非 wiki 体例")],
    "na-0009": [("c2-ip08-b15-010", "云深藏品志名剑名录：谱系词面最重，无菜谱对偶设定"),
                ("c2-ip07-b13-007", "贝克兰德平民菜谱：菜谱真实，无剑谱等价交换"),
                ("c2-ip14-b28-022", "牙的巡逻路线图：『等价交换』词面命中的巡逻图，无菜谱剑谱对偶；fans_only 不可推荐")],
    "na-0010": [("c2-ip10-b19-008", "灵质空间练习第一课：课程体近，无量子计算"),
                ("c2-ip02-b04-030", "太卜司算盘过宵夜：算具意象，无修仙入门形态")],
    "na-0011": [("c2-ip16-b32-012", "Charizard 教我的失去课：龙+教学词面近，非驯服打印机"),
                ("c2-ip13-b26-027", "Chalk and Borrowed Afternoons：教书线，无驯兽教程")],
    "na-0012": [("c2-ip06-b11-025", "社团教室旧键盘：复古电器最接近，无通讯设备复活术"),
                ("c2-ip12-b23-025", "Erised 五十冬：器物长龄叙事，非传真死灵法")],
}


# sd 三档判定（人工，依据 v2-retrieval-evidence/union.json 四配置实际浮出面）
SD_JUDGE = {
    "sd-0001": {"acceptable": [], "forbidden": [("c2-ip12-b24-037", "赛事夏天线，非学习小组与违禁羽毛笔"),
                                                ("c2-ip12-b24-045", "有求必应屋学期线，无自习小组规矩")]},
    "sd-0002": {"acceptable": [], "forbidden": [("c2-ip08-b15-008", "跨IP（魔道）书信体，仅信件词面"),
                                                ("c2-ip14-b28-031", "战后清晨群像，非家书往来体")]},
    "sd-0003": {"acceptable": ["c2-ip02-b04-030"], "forbidden": [("c2-ip02-b03-043", "锻造炉火线，非太卜观星夜")]},
    "sd-0004": {"acceptable": [], "forbidden": [("c2-ip03-b05-007", "城防公务日志，非逛展攻略帖"),
                                                ("c2-ip03-b05-021", "灯会奇谭叙事，非特展导览帖")]},
    "sd-0005": {"acceptable": [], "forbidden": [("c2-ip13-b26-038", "科技公式线，无上城家宴陷阱"),
                                                ("c2-ip13-b25-004", "档案室季勤线，无家宴场景")]},
    "sd-0006": {"acceptable": [], "forbidden": [("c2-ip09-b17-036", "跨IP（天官）物候考察，体裁同型非峡谷"),
                                                ("c2-ip03-b05-012", "开野攻略讨论，游戏向非野外考察笔记")]},
    "sd-0007": {"acceptable": ["c2-ip12-b24-012"], "forbidden": [("c2-ip12-b23-014", "课程教科书总表，非温室日志体")]},
    "sd-0008": {"acceptable": [], "forbidden": [("c2-ip16-b31-024", "中心许愿墙故事，非展柜讲解词"),
                                               ("c2-ip08-b15-046", "跨IP（魔道）同型讲解词，形态近但非宝可梦化石展")]},
    "sd-0009": {"acceptable": [], "forbidden": [("c2-ip06-b11-021", "招新剧情，非训练营体验科目"),
                                                ("c2-ip06-b12-011", "赛事周末记录，无散人体验科目")]},
    "sd-0010": {"acceptable": [], "forbidden": [("c2-ip02-b03-001", "电台栏目，无首班车通线叙事"),
                                                ("c2-ip02-b03-005", "城防巡逻志，非铁路纪事")]},
    "sd-0011": {"acceptable": ["c2-ip10-b20-018"], "forbidden": [("c2-ip10-b19-025", "食堂日常，无退休带队叙事"),
                                                                 ("c2-ip10-b19-011", "会馆年录，无退休线")]},
    "sd-0012": {"acceptable": [], "forbidden": [("c2-ip12-b24-021", "小天狼星线的夏天，非蛇院孩子日常"),
                                                ("c2-ip12-b24-037", "赛事夏天，非蛇院视角")]},
    "sd-0013": {"acceptable": ["c2-ip12-b24-003"], "forbidden": [("c2-ip12-b24-037", "世界杯夏天，非争霸赛双冠军线")]},
    "sd-0014": {"acceptable": [], "forbidden": [("c2-ip16-b31-043", "搭档讨论帖形态，非呆呆兽视角叙事"),
                                                ("c2-ip16-b32-018", "清晨巴士日常，无码头潮汐叙事者")]},
    "sd-0015": {"acceptable": [], "forbidden": [("c2-ip12-b24-003", "争霸赛公务裁判线，无开盘赌局喜剧"),
                                                ("c2-ip12-b23-025", "器物长龄叙事，无赌局设定")]},
    "sd-0016": {"acceptable": ["c2-ip07-b13-018"], "forbidden": [("c2-ip07-b14-021", "分部夜话合集，无烟道结案事件")]},
    "sd-0017": {"acceptable": ["c2-ip02-b04-031"], "forbidden": [("c2-ip02-b03-045", "双语旅行纪行，非酒店员工视角")]},
    "sd-0018": {"acceptable": ["c2-ip12-b23-017"], "forbidden": [("c2-ip12-b23-014", "课程设定总表，无传纸条叙事")]},
    "sd-0019": {"acceptable": ["c2-ip09-b18-016", "c2-ip09-b17-014"], "forbidden": [("c2-ip09-b17-050", "飞升机制讨论，非该案伏笔考据")]},
    "sd-0020": {"acceptable": ["c2-ip04-b07-005"], "forbidden": [("c2-ip08-b16-026", "跨IP（魔道）妖邪图鉴，词面重叠"),
                                                                 ("c2-ip04-b07-013", "叙事外编，非案卷档案体")]},
    "sd-0021": {"acceptable": ["c2-ip07-b13-006"], "forbidden": [("c2-ip07-b14-013", "失踪悬疑案线，非温馨小侦探")]},
    "sd-0022": {"acceptable": [], "forbidden": [("c2-ip15-b29-008", "厨房线，无睡眠规律考"),
                                                ("c2-ip15-b29-024", "群像休航日，非单一角色习惯志")]},
    "sd-0023": {"acceptable": [], "forbidden": [("c2-ip02-b04-044", "百货 AU，无钟表修复线"),
                                                ("c2-ip07-b13-027", "跨IP（诡秘）钟摆，仅钟词面")]},
    "sd-0024": {"acceptable": [], "forbidden": [("c2-ip02-b04-036", "开港日航运纪事，无宴席座次"),
                                                ("c2-ip02-b04-021", "行商押航篇，无座次礼仪考")]},
    "sd-0025": {"acceptable": ["c2-ip15-b30-019"], "forbidden": [("c2-ip15-b29-034", "悬赏金数据考据，非投递视角叙事"),
                                                                 ("c2-ip15-b30-006", "舰船航海志，无投递线")]},
    "sd-0026": {"acceptable": [], "forbidden": [("c2-ip02-b03-031", "剑的器物线，非军营成长"),
                                                ("c2-ip02-b03-043", "锻造线，无演武场叙事")]},
    "sd-0027": {"acceptable": [], "forbidden": [("c2-ip13-b25-001", "群像叙事，无声景采风手记"),
                                                ("c2-ip13-b26-031", "议会线，仅『回声』词面")]},
    "sd-0028": {"acceptable": [], "forbidden": [("c2-ip07-b13-018", "值夜办案线，无菜谱条目"),
                                                ("c2-ip07-b14-021", "分部夜话合集，无厨房线")]},
    "sd-0029": {"acceptable": ["c2-ip09-b18-015"], "forbidden": [("c2-ip09-b18-008", "风信线，非风师告示之争"),
                                                                 ("c2-ip09-b18-004", "殿务记录，无兄弟告示线")]},
    "sd-0030": {"acceptable": [], "forbidden": [("c2-ip14-b27-045", "跨IP（火影）体术讨论，仅讨论形态同型"),
                                                ("c2-ip15-b29-050", "卷评入门帖，无六式拆解")]},
    "sd-0031": {"acceptable": [], "forbidden": [("c2-ip01-b01-028", "跨IP（原神）导览手册，仅导览词面"),
                                                                 ("c2-ip09-b17-027", "跨IP（天官）旧物展导览"),
                                                                 ("c2-ip04-b08-034", "跨IP（西游）文物展导览，非封神劫后遗物展")]},
    "sd-0032": {"acceptable": ["c2-ip09-b17-031"], "forbidden": [("c2-ip09-b17-041", "灯会群像，无查账下界线"),
                                                                 ("c2-ip09-b18-024", "天界考勤簿，无下界行脚")]},
    "sd-0033": {"acceptable": [], "forbidden": [("c2-ip08-b16-043", "宴饮外篇，非展柜讲解词"),
                                                ("c2-ip08-b16-050", "观灯节庆线，无展陈讲解"),
                                                ("c2-ip03-b05-028", "跨IP（王者）特展导览帖，同型跨IP")]},
    "sd-0034": {"acceptable": ["c2-ip03-b05-009"], "forbidden": [("c2-ip03-b05-013", "观赛笔记，非己方复盘"),
                                                ("c2-ip03-b05-050", "面基赛活动帖，无逐帧复盘")]},
    "sd-0035": {"acceptable": ["c2-ip01-b02-024"], "forbidden": [("c2-ip01-b01-038", "璃月报刊合集，非学生交换日记"),
                                                ("c2-ip01-b01-017", "须弥信件线，非枫丹校园线")]},
    "sd-0036": {"acceptable": ["c2-ip15-b29-034"], "forbidden": [("c2-ip15-b30-043", "竞技场叙事，无悬赏令物质性考"),
                                                ("c2-ip15-b30-002", "茶屋便笺，无公告栏线")]},
    "sd-0037": {"acceptable": [], "forbidden": [("c2-ip01-b02-019", "书店事件单篇，无报刊栏目形态"),
                                                ("c2-ip01-b02-041", "借书理由单篇，非报纸见闻合集")]},
    "sd-0038": {"acceptable": ["c2-ip02-b04-018"], "forbidden": [("c2-ip02-b04-024", "星路商栈灯火，无渡船规矩"),
                                                ("c2-ip15-b30-029", "跨IP（海贼）旧船长诗，仅船词面")]},
    "sd-0039": {"acceptable": [], "forbidden": [("c2-ip15-b30-006", "舰队航海志，无退休离职线"),
                                                ("c2-ip15-b29-017", "船的终章叙事，无人物离职线")]},
    "sd-0040": {"acceptable": ["c2-ip09-b17-001"], "forbidden": [("c2-ip09-b17-041", "灯会群像叙事，无关系模式论"),
                                                ("c2-ip09-b17-025", "裴将军小品，非双主关系论")]},
    "sd-0041": {"acceptable": ["c2-ip03-b06-038", "c2-ip03-b06-003"], "forbidden": [("c2-ip03-b05-011", "对局叙事，无商路设定"),
                                                ("c2-ip03-b05-021", "灯会奇谭，无伏笔考据")]},
    "sd-0042": {"acceptable": ["c2-ip02-b04-013"], "forbidden": [("c2-ip16-b32-015", "跨IP（宝可梦）汽水铺，仅汽水词面"),
                                                ("c2-ip02-b04-035", "信使转投线，无酒馆柜台")]},
    "sd-0043": {"acceptable": ["c2-ip02-b04-027"], "forbidden": [("c2-ip01-b01-020", "跨IP（原神）食堂线，仅食馔词面"),
                                                ("c2-ip02-b03-015", "缉妖司公案夜话，无厨房线")]},
    "sd-0044": {"acceptable": ["c2-ip07-b13-018"], "forbidden": [("c2-ip07-b14-021", "夜话合集，非三则怪谈结案体")]},
    "sd-0045": {"acceptable": ["c2-ip02-b03-049"], "forbidden": [("c2-ip02-b04-017", "糖果车贩卖线，无乘务观察日记"),
                                                ("c2-ip02-b03-001", "电台栏目，无新乘客视角")]},
    "sd-0046": {"acceptable": [], "forbidden": [("c2-ip13-b25-004", "档案室线，无涂鸦往来"),
                                                ("c2-ip14-b28-001", "跨IP（火影）会谈前夜，词面近")]},
    "sd-0047": {"acceptable": ["c2-ip02-b04-001", "c2-ip02-b03-022"], "forbidden": [("c2-ip03-b06-013", "跨IP（王者）甜品摊，仅收摊词面"),
                                                ("c2-ip02-b04-041", "梦境餐车，无百年摊规叙事")]},
    "sd-0048": {"acceptable": ["c2-ip04-b07-030"], "forbidden": [("c2-ip04-b08-050", "年节宴叙事，非化缘斋饭谱"),
                                                ("c2-ip04-b07-005", "法宝器物谱，无食馔条目")]},
}
