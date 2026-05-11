"""OmniCraft Test Data Seeder — uses only stdlib (urllib)."""
import json, sys, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"
TOKEN = None

def api(method, path, data=None):
    url = f"{BASE}{path}"
    body = json.dumps(data).encode() if data else None
    headers = {"Content-Type": "application/json; charset=utf-8"}
    if TOKEN:
        headers["Authorization"] = f"Bearer {TOKEN}"
    req = urllib.request.Request(url, data=body, method=method.upper(), headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            resp = json.loads(r.read())
            return resp
    except urllib.error.HTTPError as e:
        err_body = ""
        try: err_body = e.read().decode()[:250]
        except: pass
        print(f"  ERR {e.code}: {err_body}")
        return None
    except Exception as e:
        print(f"  EXC: {e}")
        return None

# Login
print("=== Logging in ===")
r = api("POST", "/auth/login", {"email": "demo@omnicraft.com", "password": "demo123456"})
if not r:
    print("Login failed!")
    sys.exit(1)
TOKEN = r["tokens"]["access_token"]
UID = r["user"]["id"]
print(f"Logged in as uid={UID} role={r['user']['role']}")

# === IPs ===
ip_data = [
    ("原神", "米哈游开发的开放世界冒险游戏，以提瓦特大陆为舞台。", "gaming", ["开放世界","RPG","米哈游"]),
    ("崩坏：星穹铁道", "米哈游科幻题材回合制RPG，星际旅行和银河冒险。", "gaming", ["回合制","RPG","科幻","米哈游"]),
    ("鬼灭之刃", "吾峠呼世晴创作的少年漫画，灶门炭治郎的故事。", "animation", ["少年漫画","热血","和风"]),
    ("流浪地球", "刘慈欣原著、郭帆导演的科幻电影系列。", "film_tv", ["科幻","电影","刘慈欣"]),
    ("明日方舟", "鹰角网络开发的策略塔防手游，末日废土世界观。", "gaming", ["塔防","策略","末世"]),
    ("艾尔登法环", "FromSoftware开放世界动作RPG，宫崎英高×乔治RR马丁。", "gaming", ["魂系","开放世界","奇幻"]),
    ("三体", "刘慈欣创作的硬科幻小说系列，地球文明与三体文明的接触。", "novel", ["科幻","小说","刘慈欣"]),
    ("原神-枫丹", "原神第四国度，水之国枫丹，蒸汽朋克与魔术的世界。", "gaming", ["原神","枫丹","蒸汽朋克"]),
]

ips = {}
print("\n=== Creating IPs ===")
for name, desc, cat, tags in ip_data:
    r = api("POST", "/ips", {"name": name, "description": desc, "category": cat, "tags": tags})
    if r:
        iid = r["ip"]["id"]
        status = r["ip"]["status"]
        ips[name] = iid
        print(f"  [{iid}] {name} ({status})")
        if status == "pending":
            ar = api("POST", f"/admin/ips/{iid}/approve")
            if ar:
                print(f"    -> approved")

# === Original Contents ===
originals = {}
orig_data = [
    ("《流浪地球3》首批剧照深度解析：行星发动机设计美学",
     "## 发动机的工业美学\n\n最近《流浪地球3》放出了首批官方剧照，行星发动机终于有了正面特写。\n\n### 设计细节\n\n1. 进气口格栅参考了航空发动机的涡轮结构\n2. 外壳的铆钉排列参考了苏联重工业时代的设计\n3. 整体配色带有蓝色能量光纹的深灰\n\n> 我们想要一种既有工业力量感，又不会让人觉得遥不可及的设计\n\n你们觉得新设计怎么样？",
     "film_tv", "image", ["流浪地球","科幻电影","概念设计"]),
    ("《黑神话：悟空》隐藏BOSS青背龙无伤打法全攻略",
     "## 前言\n\n花了整整三天研究青背龙的无伤打法。\n\n### 装备推荐\n\n- 业火杖：火属性克制龙类\n- 锦斓袈裟：高雷抗+自动回血\n- 定风珠：免疫龙卷风控场\n\n### 一阶段\n左爪高举则落雷，左爪横扫则雷刃，双爪合拢则全屏雷暴。\n\n祝各位天命人武运昌隆！",
     "gaming", "image", ["黑神话悟空","BOSS攻略","动作游戏"]),
    ("复刻《孤独的美食家》五郎同款蒜香黄油煎饭团",
     "## 五郎叔吃了都说好\n\n昨晚重温《孤独的美食家》，今天立刻复刻了一版。\n\n### 食材：冷米饭300g、黄油20g、大蒜4瓣、酱油1大勺、味淋1小勺、海苔2片、白芝麻\n\n1. 冷米饭捏成饭团，捏紧实但不捏碎米粒\n2. 黄油融化，蒜片煸至金黄\n3. 饭团入锅，两面煎3分钟至焦脆\n4. 酱油+味淋刷表面，再煎30秒上色\n\n外酥内软，黄油的奶香混着蒜香——**真的太好吃了！**",
     "food", "image", ["美食复刻","日料","饭团"]),
    ("捡到一只流浪橘猫的第30天：从纸片猫到煤气罐的蜕变",
     "## Day 1\n在小区垃圾桶旁边发现它的时候，瘦得只剩一把骨头。\n## Day 7\n开始主动进食了！\n## Day 15\n第一次主动蹭我的手！！\n## Day 30\n胖了整整1.8kg！名字取好了，叫八筒——鼻子上有个八字花纹。欢迎来到新家！",
     "pet", "image", ["橘猫","流浪猫救助","养猫日记"]),
    ("短篇科幻 | 《最后的信使》",
     "## 一\n\n2147年的太阳和2025年的没有区别。\n\n林远站在月球基地的全景窗前。最后的传输窗口还有三分钟。\n\n## 二\n\n给未来的人类：如果你们收到这封信，说明我们失败了。\n\n我们在月球背面发现的信标，在激活后用了不到48小时就瓦解了地球的整个通讯网络。不是攻击，更像是覆盖。\n\n## 三\n\n信标是一面镜子。所有的信息都在被它吸进去，然后吞掉。我们不再能互相听到了。\n\n## 四\n\n林远按下发送键。他不知道这封信能否到达比邻星。但他还是写了。因为总要有人留下点什么。",
     "literature", "article", ["科幻","短篇","原创小说"]),
    ("自驾318川藏线全攻略：从成都到拉萨的2000公里",
     "## 路线概览\n\n历时14天，总里程2140km，最高海拔5130m。\n\n### Day 1-3 成都到理塘\n新都桥到理塘之间加油站非常少。\n\n### Day 4-6 稻城亚丁\n亚丁三神山太壮观了！央迈勇的雪峰在晨光中呈现金色倒影。\n\n### Day 7-10 理塘到林芝\n怒江72拐是整个行程最刺激的路段。\n\n### Day 11-14 林芝到拉萨\n布达拉宫出现在地平线上的那一刻，所有的疲惫都值得了。",
     "travel", "image", ["川藏线","自驾游","318国道"]),
    ("MacBook Pro M5 Max 深度评测：性能与续航的完美平衡",
     "## 开箱第一印象\n\n新的深空黑色真的太高级了！\n\n### 跑分数据\n- Geekbench多核 26800（M4 Max 22100，+21%）\n- Cinebench GPU 18200（+20%）\n\n### 实际体验\n- 视频剪辑：8K ProRes实时预览无压力\n- 3D渲染：Blender BMW场景仅需47秒\n- 续航：正常办公18小时\n\n**总结**：性能天花板+续航王者，创意工作者闭眼入。",
     "tech_digital", "video", ["MacBook","苹果","评测"]),
    ("15平出租屋改造：从城中村风到日系治愈小家",
     "## 改造清单\n\n- 米色墙纸 280元、暖色落地灯 169元、木质置物架x2 320元\n- 棉麻窗帘 135元、床品四件套 299元、挂画绿植 200元、地毯 189元\n\n### 核心思路\n1. 统一色调：米色+浅木色+绿植点缀\n2. 灯光层次：主灯+落地灯+床头灯\n3. 垂直收纳：能上墙的都上墙\n4. 留白：再小的空间也要留出呼吸感的角落\n\n改造是一件特别有成就感的事情。",
     "home", "image", ["出租屋改造","家居","日系"]),
    ("2026春夏穿搭趋势：5套低饱和度叠穿LOOK分享",
     "## LOOK 1: 燕麦色西装+垂感阔腿裤\n\n燕麦色是本季最重要的基础色，搭配同色系阔腿裤，慵懒但不邋遢。\n\n## LOOK 2: 鼠尾草绿针织+白色百褶裙\n\n鼠尾草绿低饱和度非常适合叠穿。\n\n你们更喜欢哪一套？",
     "beauty_fashion", "image", ["穿搭","春夏","OOTD"]),
    ("马拉松训练计划Notion模板分享：从5km到全马",
     "## 模板简介\n\n花了三个月打磨的马拉松训练模板：\n\n1. 16周训练周期：每周训练计划自动生成\n2. 配速计算器：根据目标成绩自动计算配速\n3. 跑量追踪：周跑量/月跑量统计+图表\n4. 营养日记：赛前碳负荷计算器\n5. 伤病记录：常见跑步伤病自查表",
     "sports", "template", ["马拉松","跑步","Notion模板"]),
    ("我用AI工具搭建了个人知识管理系统，效率提升3倍",
     "## 工具栈\n\n- 信息收集：Cubox + Readwise\n- 整理归档：Obsidian（双向链接+AI自动标签）\n- 输出创作：Claude（整理思路到扩写草稿到润色）\n\n### 核心原则\n\n1. 不追求完美：笔记不需要完整，能触发回忆就够了\n2. AI辅助整理，人来判断\n3. 定期回顾：每周花30分钟回顾本周笔记",
     "productivity", "article", ["效率","知识管理","AI工具"]),
]

print("\n=== Creating Original Contents ===")
for title, desc, cat, ctype, tags in orig_data:
    r = api("POST", "/contents", {
        "title": title, "description": desc, "zone": "original",
        "category": cat, "content_type": ctype, "tags": tags
    })
    if r:
        oid = r.get("content", r).get("id")
        originals[title[:30]] = oid
        print(f"  [{oid}] {title[:55]}... ({ctype})")

# === Fanworks ===
fanwork_data = [
    ("雷电将军一心净土同人插画",
     "一直想画雷电将军在一心净土中的场景。紫色和金色对比来表现她内心的矛盾。工具：iPad Pro + Procreate，分辨率：4000x6000px。希望大家喜欢！",
     "原神", "image", ["同人插画","雷电将军","电绘"], None),
    ("星穹同人 | 银河列车没有终点站",
     "## 第一章\n\n姬子关掉了通讯器。列车的窗外是一片她从未见过的星云。\n\n还是没有信号？三月七靠在门框上。\n\n姬子摇了摇头。所有的频道都是同一个声音——一个巨大生物的心跳。\n\n前方的紫色星云中，一个巨大的轮廓正在缓缓浮现。\n\n未完待续。同人创作，版权归米哈游所有。",
     "崩坏：星穹铁道", "article", ["同人文","科幻","星穹铁道"], "短篇科幻 | 《最后的信使》"),
    ("鬼灭之刃MAD炭治郎之歌×红莲华混剪",
     "用了两周时间剪出来的鬼灭MAD。素材覆盖第一季+无限列车BD，剪辑软件DaVinci Resolve 18。BGM：LiSA - 红莲华。大哥永远活在我们心里。",
     "鬼灭之刃", "video", ["MAD","鬼灭之刃","燃向","混剪"], None),
    ("明日方舟阿米娅升变4K壁纸级插画",
     "阿米娅升变形态是在方舟里最喜欢的设计。构图采用仰望视角——阿米娅站在甲板上，背后是移动城邦剪影。分辨率：3840x2160，工具：Clip Studio Paint EX，耗时：16小时。",
     "明日方舟", "image", ["阿米娅","插画","壁纸"], None),
    ("原神角色胡桃Stable Diffusion LoRA模型分享",
     "训练了胡桃的角色LoRA。推荐参数：Model Anything V5, LoRA weight 0.75, Sampler DPM++ 2M Karras, Steps 28, CFG 7。脸部还原度高，服装还原完美。",
     "原神", "prompt", ["LoRA","SD","胡桃","AI绘画"], None),
    ("流浪地球KSP行星发动机MOD v1.2",
     "在KSP中重现流浪地球的行星发动机！包含行星发动机零件3种规格、地壳锚固装置、地下城模块、领航员号空间站预制文件。解压到GameData/，需ModuleManager 4.2+。",
     "流浪地球", "mod", ["KSP","MOD","太空"], None),
    ("鬼灭之刃灶门炭治郎之歌钢琴独奏谱",
     "扒了鬼灭第19集那首让人泪崩的插曲。中级难度（英皇5-6级），降E大调，Andante速度。左手琶音保持均匀力度，副歌右手八度跳跃需提前准备。希望你们弹的时候不会哭。",
     "鬼灭之刃", "sheet_music", ["乐谱","钢琴","OST"], None),
    ("银狼黑客帝国赛博朋克风格插画",
     "银狼太适合赛博朋克风格了！背景设定在星穹列车的下层甲板，霓虹灯管和全息广告构成了混乱但美丽的背景。画面中隐藏了多处彩蛋——右下角有三月七的Wanted海报。",
     "崩坏：星穹铁道", "image", ["赛博朋克","银狼","插画"], None),
    ("原神枫丹水下探索指南：全寻宝点与隐藏成就攻略",
     "枫丹水下探索是整个4.0版本最大的亮点。游泳加速消耗水之印，触碰水母补充。隐藏洞口通常被海草遮挡，用元素视野发现。共38个水下宝箱+3个隐藏成就需要特定角色触发！",
     "原神-枫丹", "article", ["枫丹","攻略","原神","探索"], None),
]

print("\n=== Creating Fanworks ===")
for title, desc, ip_name, ctype, tags, src_key in fanwork_data:
    body = {"title": title, "description": desc, "zone": "fanwork",
            "ip_id": ips[ip_name], "content_type": ctype, "tags": tags}
    if src_key:
        for k, v in originals.items():
            if k.startswith(src_key):
                body["source_original_id"] = v
                break
    r = api("POST", "/contents", body)
    if r:
        fid = r.get("content", r).get("id")
        print(f"  [{fid}] {title[:55]}... ({ctype}) @ {ip_name}")

print(f"\n=== Done ===")
print(f"IPs: {len(ips)} | Originals: {len(originals)} | Fanworks: {len(fanwork_data)}")
print(f"Total contents: {len(originals)+len(fanwork_data)}")
