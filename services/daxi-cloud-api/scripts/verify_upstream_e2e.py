#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DAXI <-> Supchuang(corefusion) video-agent 上游契约端到端验证 (P1)。

【已按真实上游契约校正】(依据 corefusion-src 源码):
  - 响应统一信封:{"success":bool,"message":str,"data":{...}}(common/gin.go ApiSuccess)
  - drafts/generate 必填 product_name + selling_points(controller/video_agent.go:2406)
  - draft 不直接回 usage.quota,只回 estimate(货币口径 estimated_amount / *_usd)
  - 真实 quota 在【任务】上(estimated_quota / actual_quota,quota 单位)
  - 上游【不消费】X-Reseller-*,扣费只认上游账号(c.GetInt("id"))→ 归因只能总量级对账

本脚本直连上游,复刻 DAXI 发出的请求(Bearer + X-Reseller-* + /api/agents/video/*),
拆信封、打印真实 data、从 estimate 反推 quota 量级以校准 EST_QUOTA。

用法:
  RESELLER_KEY=sk-xxx python3 verify_upstream_e2e.py            # 只验 draft(不建视频任务,基本不产生计费)
  RESELLER_KEY=sk-xxx RUN_TASK=1 python3 verify_upstream_e2e.py # 跑完整链路(confirm/tasks/poll/final,真实预扣计费)

主要环境变量(其余见代码):
  RESELLER_KEY(必填) / UPSTREAM_BASE(默认 https://supchuang.com) / RESELLER_CODE(默认 daxi-cloud)
  RUN_TASK(默认0) / CONFIRM_DRAFT(默认1) / POLL_MAX(默认40) / POLL_INTERVAL(默认6) / TIMEOUT(默认60)
  PRODUCT_NAME / SELLING_POINTS / SCENE / PLATFORM / ASPECT_RATIO / DURATION / TARGET_AUDIENCE / STYLE / TONE
  QUOTA_PER_USD(默认500000,new-api 标准换算,用于把货币估价反推 quota 量级)
  EST_DRAFT_QUOTA(默认80000) / EST_TASK_QUOTA(默认2500000)  仅用于量级对比提示
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

KEY = os.environ.get("RESELLER_KEY", "").strip()
BASE = os.environ.get("UPSTREAM_BASE", "https://supchuang.com").rstrip("/")
if BASE.endswith("/v1"):
    BASE = BASE[:-3]
RESELLER_CODE = os.environ.get("RESELLER_CODE", "daxi-cloud")
RUN_TASK = os.environ.get("RUN_TASK", "0") == "1"
CONFIRM_DRAFT = os.environ.get("CONFIRM_DRAFT", "1") == "1"
POLL_MAX = int(os.environ.get("POLL_MAX", "40"))
POLL_INTERVAL = float(os.environ.get("POLL_INTERVAL", "6"))
TIMEOUT = float(os.environ.get("TIMEOUT", "60"))
QUOTA_PER_USD = float(os.environ.get("QUOTA_PER_USD", "500000"))
EST_DRAFT_QUOTA = int(os.environ.get("EST_DRAFT_QUOTA", "80000"))
EST_TASK_QUOTA = int(os.environ.get("EST_TASK_QUOTA", "2500000"))

# 真实 draft 请求体(可用环境变量覆盖)
DRAFT_BODY = {
    "product_name": os.environ.get("PRODUCT_NAME", "便携保温杯"),
    "selling_points": os.environ.get("SELLING_POINTS", "316不锈钢内胆;24小时保温;一键弹盖;便携防漏"),
    "target_audience": os.environ.get("TARGET_AUDIENCE", "通勤白领、学生"),
    "scene": os.environ.get("SCENE", "ecommerce"),
    "platform": os.environ.get("PLATFORM", "douyin"),
    "aspect_ratio": os.environ.get("ASPECT_RATIO", "9:16"),
    "duration": int(os.environ.get("DURATION", "15")),
    "style": os.environ.get("STYLE", "活泼"),
    "tone": os.environ.get("TONE", "种草"),
    # 上游必须有合法、已配定价的视频模型,否则任务终态 FAILURE("video model is empty")。
    "video_model": os.environ.get("VIDEO_MODEL", "doubao-seedance-2-0-260128"),
}
SCENARIO_HDR = os.environ.get("SCENARIO", "ecommerce-video")
PROBE_CUSTOMER_ID = "DXC-P1PROBE"
PROBE_KEY_ID = "DXK-P1PROBE"

results = []


def record(code, status, detail):
    results.append((code, status, detail))
    tag = {"PASS": "[PASS]", "FAIL": "[FAIL]", "MANUAL": "[MANUAL]", "SKIP": "[SKIP]", "WARN": "[WARN]"}[status]
    print(f"  {tag} {code}: {detail}")


def section(t):
    print(f"\n=== {t} ===")


def req_id():
    return "req-" + time.strftime("%Y%m%d%H%M%S", time.gmtime()) + f".{int(time.time()*1e9)%1_000_000_000:09d}"


def call(method, path, body=None, with_auth=True):
    url = BASE + "/api" + path
    data = json.dumps(body).encode("utf-8") if body is not None else None
    rid = req_id()
    headers = {
        "X-Reseller-Code": RESELLER_CODE,
        "X-Reseller-Customer-ID": PROBE_CUSTOMER_ID,
        "X-Reseller-Key-ID": PROBE_KEY_ID,
        "X-Reseller-Scenario": SCENARIO_HDR,
        "X-Reseller-Request-ID": rid,
    }
    if with_auth:
        headers["Authorization"] = "Bearer " + KEY
    if data is not None:
        headers["Content-Type"] = "application/json"
    print(f"    -> {method} {url}  (req={rid})")
    r = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(r, timeout=TIMEOUT) as resp:
            raw = resp.read().decode("utf-8", "replace")
            return resp.status, try_json(raw), raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        return e.code, try_json(raw), raw
    except Exception as e:
        return None, None, f"{type(e).__name__}: {e}"


def try_json(raw):
    try:
        return json.loads(raw)
    except Exception:
        return None


def unwrap(js):
    """返回 (success_bool, message, data)。非信封结构则原样回 (None, '', js)。"""
    if isinstance(js, dict) and "success" in js:
        return bool(js.get("success")), js.get("message", ""), js.get("data")
    return None, "", js


def deep_find(obj, names):
    """在嵌套 dict/list 里深度优先找第一个 key in names 的值(用于稳健提取未知形态字段)。"""
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k in names and isinstance(v, (int, float, str)) and v != "":
                return v
        for v in obj.values():
            r = deep_find(v, names)
            if r is not None:
                return r
    elif isinstance(obj, list):
        for v in obj:
            r = deep_find(v, names)
            if r is not None:
                return r
    return None


def dump(label, data):
    try:
        s = json.dumps(data, ensure_ascii=False, indent=2)
    except Exception:
        s = str(data)
    if len(s) > 1600:
        s = s[:1600] + f"\n... (+{len(s)-1600} chars 截断)"
    print(f"    {label}:\n" + "\n".join("      " + ln for ln in s.splitlines()))


def quota_from_usd(usd):
    return int(round(usd * QUOTA_PER_USD)) if usd else 0


def main():
    print("DAXI <-> Supchuang(corefusion) video-agent 上游契约端到端验证 (P1, 按真实契约)")
    print(f"  URL 前缀  : {BASE}/api/agents/video/*")
    print(f"  归因      : code={RESELLER_CODE} customer={PROBE_CUSTOMER_ID} key={PROBE_KEY_ID}")
    print(f"  RUN_TASK  : {RUN_TASK}")
    if not KEY:
        print("\n[ABORT] 缺少 RESELLER_KEY。")
        sys.exit(2)

    # C0 鉴权
    section("C0 鉴权生效(无 Bearer 期望 401/403)")
    code, _, raw = call("POST", "/agents/video/drafts/generate", body=DRAFT_BODY, with_auth=False)
    if code in (401, 403):
        record("C0", "PASS", f"无 Bearer 返回 {code}")
    elif code is None:
        record("C0", "FAIL", f"网络错误: {raw}")
    else:
        record("C0", "WARN", f"无 Bearer 返回 {code}")

    # C1 端点 + 鉴权 + 请求体被接受
    section("C1 drafts/generate(真实 body:product_name+selling_points)")
    code, js, raw = call("POST", "/agents/video/drafts/generate", body=DRAFT_BODY)
    if code is None:
        record("C1", "FAIL", f"网络错误: {raw}")
        finish(); return
    print(f"    <- HTTP {code}")
    success, msg, data = unwrap(js)
    if code == 404:
        record("C1", "FAIL", "404:端点不存在"); finish(); return
    if code in (401, 403) or success is False and "登录" in str(msg):
        record("C1", "FAIL", f"鉴权被拒:{msg}"); finish(); return
    if success is False:
        record("C1", "FAIL", f"上游业务报错(信封 success=false):{msg}")
        dump("整体响应", js); finish(); return
    if success is None:
        record("C1", "WARN", f"响应非标准信封(无 success 字段),原样:{raw[:300]}")
    else:
        record("C1", "PASS", f"success=true,请求体被接受")
    dump("data(真实 draft 响应)", data)

    # C2 拆信封后能否拿到计费信息(estimate)
    section("C2 计费信息(draft 走 estimate 货币口径,非 usage.quota)")
    est = data.get("estimate") if isinstance(data, dict) else None
    draft_obj = data.get("draft") if isinstance(data, dict) else None
    draft_id = draft_obj.get("id") if isinstance(draft_obj, dict) else None
    draft_status = draft_obj.get("status") if isinstance(draft_obj, dict) else None
    print(f"    draft.id={draft_id}  draft.status={draft_status}")
    derived_quota = None
    if isinstance(est, dict):
        amount = est.get("estimated_amount")
        usd = est.get("estimated_amount_usd")
        cur = est.get("currency")
        is_billable = est.get("is_billable")
        derived_quota = quota_from_usd(usd) if usd else None
        record("C2", "PASS",
                f"estimate: amount={amount}{cur} usd={usd} is_billable={is_billable} "
                f"require_confirm={est.get('require_confirm')} needs_review={est.get('needs_manual_review')}")
    else:
        record("C2", "WARN", "data 内无 estimate 字段,见上方 data dump")

    # C4 量级对比(用货币反推的 quota 对 EST_DRAFT_QUOTA)
    section("C4 quota 量级 vs EST 占位(由 estimated_amount_usd × QUOTA_PER_USD 反推)")
    if derived_quota:
        record("C4", "PASS",
                f"反推 task 级 quota≈{derived_quota}(=${est.get('estimated_amount_usd')}×{int(QUOTA_PER_USD)});"
                f"对比 EST_TASK_QUOTA={EST_TASK_QUOTA} → {'同量级' if 0.1<=derived_quota/EST_TASK_QUOTA<=10 else '量级偏差大,建议校准'}")
    else:
        record("C4", "SKIP", "estimate 无 *_usd,无法反推;可 RUN_TASK=1 取真实 estimated_quota/actual_quota")

    if RUN_TASK:
        run_task_chain(draft_id, draft_status)
    else:
        record("C6", "SKIP", "RUN_TASK!=1,跳过 confirm/tasks/poll/final(真实预扣计费)")

    section("C3 X-Reseller-* 归因(上游源码确认:不消费 reseller 头)")
    record("C3", "MANUAL",
           "上游 corefusion 不读 X-Reseller-*,用量全记在 DAXI 的上游账号名下 → "
           "Supchuang 报表查不到 DXC-P1PROBE;对账只能在【上游账号总量级】。"
           "若要按 DAXI 客户拆分,需上游加 reseller 头消费(上游侧改造)。")
    finish()


def run_task_chain(draft_id, draft_status):
    if not draft_id:
        record("C6", "FAIL", "无 draft.id,无法建任务"); return
    print(f"\n    draft_id={draft_id} status={draft_status}")

    if CONFIRM_DRAFT:
        section("C5 confirm draft")
        code, js, raw = call("POST", f"/agents/video/drafts/{draft_id}/confirm", body={})
        s, m, d = unwrap(js)
        if code and 200 <= code < 300 and s is not False:
            record("C5", "PASS", f"confirm ok (status={deep_find(d, {'status'})})")
        else:
            record("C5", "WARN", f"confirm HTTP {code} success={s} msg={m}")

    section("C6 建任务(真实预扣 quota)")
    code, js, raw = call("POST", f"/agents/video/drafts/{draft_id}/tasks", body={})
    s, m, d = unwrap(js)
    if not (code and 200 <= code < 300) or s is False:
        record("C6", "FAIL", f"建任务失败 HTTP {code} success={s} msg={m}")
        dump("响应", js); return
    dump("data(建任务响应)", d)
    # 上游任务有两个 id:int `id` 与 string `task_id`;轮询/状态必须用 string task_id。
    task_id = (d.get("task_id") if isinstance(d, dict) else None) or deep_find(d, {"task_id", "TaskID"})
    est_quota = deep_find(d, {"estimated_quota"})
    record("C6", "PASS", f"task_id={task_id} estimated_quota={est_quota} status={deep_find(d, {'status'})}")
    if est_quota:
        ratio = est_quota / EST_TASK_QUOTA if EST_TASK_QUOTA else 0
        record("C4b", "PASS" if 0.1 <= ratio <= 10 else "WARN",
               f"真实 estimated_quota={est_quota} vs EST_TASK_QUOTA={EST_TASK_QUOTA} (ratio={ratio:.2f})")
    if not task_id:
        record("C7", "FAIL", "无 task_id,无法轮询"); return

    section("C7 轮询任务至终态 + 真实 quota")
    terminal = {"success", "completed", "complete", "failed", "error", "canceled", "cancelled"}
    final = None
    for i in range(POLL_MAX):
        code, js, raw = call("GET", f"/agents/video/tasks/{task_id}")
        s, m, d = unwrap(js)
        st = deep_find(d, {"status"})
        aq = deep_find(d, {"actual_quota"})
        print(f"    [{i+1}/{POLL_MAX}] status={st} actual_quota={aq}")
        if isinstance(st, str) and st.lower() in terminal:
            final = (st, d); break
        time.sleep(POLL_INTERVAL)
    if not final:
        record("C7", "FAIL", f"{POLL_MAX} 次未到终态"); return
    st, d = final
    aq = deep_find(d, {"actual_quota"})
    dump("data(终态任务)", d)
    if st.lower() in {"failed", "error", "canceled", "cancelled"}:
        record("C7", "WARN", f"终态={st}(非成功) actual_quota={aq}")
    else:
        record("C7", "PASS", f"终态={st} actual_quota={aq}")

    section("C8 final-video")
    code, js, raw = call("GET", f"/agents/video/tasks/{task_id}/final-video")
    s, m, d = unwrap(js)
    url = deep_find(d, {"result_url", "url", "video_url", "final_url"})
    if code and 200 <= code < 300 and s is not False:
        record("C8", "PASS", f"ok result_url={url}")
    else:
        record("C8", "WARN", f"HTTP {code} success={s} msg={m}")


def finish():
    section("汇总")
    order = {"FAIL": 0, "WARN": 1, "MANUAL": 2, "SKIP": 3, "PASS": 4}
    for code, status, detail in sorted(results, key=lambda x: order[x[1]]):
        print(f"  {status:<7} {code:<4} {detail}")
    fails = [r for r in results if r[1] == "FAIL"]
    warns = [r for r in results if r[1] == "WARN"]
    print()
    if fails:
        print(f"结论:有 {len(fails)} 项 FAIL。")
        sys.exit(1)
    print(f"结论:契约连通(含 {len(warns)} 项告警/人工项)。C3 归因务必人工确认。")
    sys.exit(0)


if __name__ == "__main__":
    main()
