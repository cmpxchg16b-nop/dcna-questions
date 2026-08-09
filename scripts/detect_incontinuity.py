import re

path = "/mnt/agents/upload/user_pasted_clipboard_long_content_as_file_#correct=4, num=1 二等.txt"

# 解析每行的 num
entries = []  # (行号, num, 行内容)
with open(path, encoding="utf-8") as f:
    for lineno, line in enumerate(f, 1):
        m = re.match(r"#correct=\d+,\s*num=(\d+)\s+(.*)", line.strip())
        if m:
            entries.append((lineno, int(m.group(1)), m.group(2)))
        elif line.strip():
            print(f"[警告] 第 {lineno} 行无法解析: {line.strip()[:60]}")

# num 变小（重置为更小的值）即视为新 section 的开始
sections = []  # 每个 section 是 entries 的子列表
current = []
for e in entries:
    if current and e[1] <= current[-1][1]:  # num 未递增 → 新 section
        sections.append(current)
        current = []
    current.append(e)
if current:
    sections.append(current)

print(f"共解析 {len(entries)} 行，识别出 {len(sections)} 个 section\n")

# 逐 section 检查连续性
for i, sec in enumerate(sections, 1):
    nums = [n for _, n, _ in sec]
    start, end = nums[0], nums[-1]
    expected = set(range(start, end + 1))
    missing = sorted(expected - set(nums))
    # 重复
    seen, dup = set(), set()
    for n in nums:
        if n in seen:
            dup.add(n)
        seen.add(n)
    status = "连续" if not missing and not dup else "存在问题"
    print(f"Section {i}: 行 {sec[0][0]}~{sec[-1][0]}, num {start}~{end}, 共 {len(nums)} 题 → {status}")
    if missing:
        print(f"   缺失: {missing}")
        for m_ in missing:
            # 找到缺失位置前后的题目，方便定位
            prev = max((x for x in sec if x[1] < m_), key=lambda x: x[1], default=None)
            nxt = min((x for x in sec if x[1] > m_), key=lambda x: x[1], default=None)
            print(f"      num={m_} 应位于 第{prev[0]}行[num={prev[1]}]「{prev[2][:25]}…」 与 第{nxt[0]}行[num={nxt[1]}]「{nxt[2][:25]}…」 之间")
    if dup:
        print(f"   重复: {sorted(dup)}")