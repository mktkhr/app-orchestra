#!/usr/bin/env python3
"""Latency microbenchmark: (A) Jev batch-size curve, (B) concurrency for
Jev / local pick / local end-to-end. Run with:

    TYPESAFE_KEY="<key>" python3 latency-bench.py

The key is read only from the TYPESAFE_KEY env var - never written to
disk and never logged.
"""

import concurrent.futures
import json
import os
import sys
import time
import urllib.error
import urllib.request

API_KEY = os.environ.get("TYPESAFE_KEY")
if not API_KEY:
    print("TYPESAFE_KEY not set in environment", file=sys.stderr)
    sys.exit(1)

JEV_URL = "https://api.typesafe.ai/v1/systemone"
LOCAL_PICK_URL = "http://localhost:11435/v1/chat/completions"
LOCAL_SESSION_URL = "http://localhost:8080/api/session"
LOCAL_PLAN_URL = "http://localhost:8080/api/plan"

OUT_DIR = os.path.dirname(os.path.abspath(__file__))
BATCH_JSONL = os.path.join(OUT_DIR, "latency-batch-jev.jsonl")
CONC_JSONL = os.path.join(OUT_DIR, "latency-concurrency.jsonl")
SIZE13_JSON = os.path.join(OUT_DIR, "latency-batch-request-size13.json")

STATE = "在庫の一覧を見せて"

PICK_QUESTION = {
    "type": "choice",
    "instructions": (
        "社内APIの振り分け役。質問に対して、候補の中から呼ぶべき操作を1つ選ぶ。"
        "list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に"
        "何かを出したい質問のとき、noneはどの候補も質問に合わない、または質問が業務と"
        "無関係なときに選ぶ。候補には examples（その操作に対して人がよく尋ねる質問）と"
        "not_for（混同しやすい別の操作）がある。"
    ),
    "criteria": {
        "CreateAttendanceRecord": {
            "what": (
                "勤怠管理 / Create an attendance record.。Adds a new attendance "
                "record to the service's in-memory store. Its id is assigned by "
                "the service."
            ),
            "examples": ["勤怠記録を登録して", "振替休日を申請したい"],
        },
        "CreateInventoryItem": {
            "what": (
                "在庫管理 / Create a stock item.。Adds a new stock item to the "
                "service's in-memory store. Its id is assigned by the service."
            ),
            "examples": ["新しい在庫アイテムを登録して", "在庫を追加したい"],
        },
        "GetAttendanceRecord": {
            "what": (
                "勤怠管理 / Get one attendance record by id.。Returns a single "
                "attendance record, or 404 when the id is unknown."
            ),
            "examples": ["この記録の詳細を教えて", "この勤怠の中身を見たい"],
        },
        "GetInventoryItem": {
            "what": (
                "在庫管理 / Get one stock item by id.。Returns a single stock "
                "item, or 404 when the id is unknown."
            ),
            "examples": ["このアイテムの詳細を教えて", "この在庫の中身を見たい"],
        },
        "ListAttendanceRecords": {
            "what": (
                "勤怠管理 / List attendance records, optionally filtered by "
                "kind.。Returns every attendance record known to the service. "
                "The optional kind query parameter narrows the result to "
                "records of that kind."
            ),
            "examples": ["勤怠記録を見せて", "今月の出勤状況を教えて", "振替休日の記録だけ見たい"],
        },
        "ListInventoryItems": {
            "what": (
                "在庫管理 / List stock items, optionally filtered by status.。"
                "Returns every stock item known to the service. The optional "
                "status query parameter narrows the result to items in that "
                "status."
            ),
            "examples": ["在庫を見せて", "今の在庫状況を教えて", "引当済の在庫だけ見たい"],
        },
        "list_capabilities": {
            "what": "使える操作の一覧を知りたい",
            "examples": ["何ができるの？"],
        },
        "none": {
            "what": "どの候補も質問に合わない（業務と無関係な質問）",
            "examples": ["今日の天気は？", "好きな食べ物は何？", "システムを再起動して"],
        },
        "propose_panel": {"what": "画面に出したい"},
    },
}

# Extra questions appended in this fixed order as batch size grows. #1 is
# the exact "impossible" noul from raw/jev-fanout-gate-request.json.
EXTRA_QUESTIONS = [
    (
        "impossible",
        {
            "type": "noul",
            "instructions": (
                "この質問は、列挙された候補のどれでも実現できないことを求めているか"
                "（例: 一覧しかない資源の集計・承認・印刷、候補に無い資源、業務と無関係な"
                "話題）。能力を尋ねる質問（何ができる？）はfalse。"
            ),
            "criteria": {
                "false": "候補のどれかで答えられる、または「何ができるか」を尋ねている",
                "true": (
                    "質問が求める操作が候補一覧に無い（一覧しかない資源の集計・承認・"
                    "印刷、一覧に無い資源、業務と無関係）"
                ),
            },
        },
    ),
    (
        "cancel",
        {
            "type": "noul",
            "instructions": "質問は取り消せない操作（削除・確定・送信など、元に戻せない変更）を求めているか。",
            "criteria": {"false": "取り消せる操作、または閲覧のみ", "true": "取り消せない操作を求めている"},
        },
    ),
    (
        "panel",
        {
            "type": "noul",
            "instructions": "質問は画面に何かを出したい依頼か（データの一覧・詳細を画面表示したいという意味の依頼）。",
            "criteria": {"false": "画面表示以外の依頼、または依頼ではない", "true": "画面に出したい依頼"},
        },
    ),
    (
        "date",
        {
            "type": "noul",
            "instructions": "質問に日付や期間の条件（今月、今日、いつからいつまで、など）が含まれるか。",
            "criteria": {"false": "日付・期間の条件は含まれない", "true": "日付・期間の条件が含まれる"},
        },
    ),
    (
        "urgent",
        {
            "type": "noul",
            "instructions": "質問は急ぎの対応を求めているか（至急、今すぐ、などの表現の有無）。",
            "criteria": {"false": "急ぎの表現はない", "true": "急ぎの表現がある"},
        },
    ),
    (
        "filter",
        {
            "type": "noul",
            "instructions": "質問に絞り込み条件（特定のステータス・種類・ラベルだけを見たい、など）があるか。",
            "criteria": {"false": "絞り込み条件はない（全件を指している）", "true": "絞り込み条件がある"},
        },
    ),
    (
        "ambiguity_score",
        {
            "type": "score",
            "instructions": "質問の曖昧さの度合い。",
            "criteria": [
                "曖昧さがない。候補を一意に決められる",
                "やや曖昧。複数の候補が考えられる",
                "強く曖昧。候補を1つに絞れない",
            ],
        },
    ),
    (
        "multi_op",
        {
            "type": "noul",
            "instructions": "質問は複数の操作にまたがる依頼か（一度に複数の異なる操作を求めている）。",
            "criteria": {"false": "単一の操作で答えられる", "true": "複数の操作にまたがる"},
        },
    ),
    (
        "approval",
        {
            "type": "noul",
            "instructions": "質問は承認を必要とする操作を求めているか。",
            "criteria": {"false": "承認は不要", "true": "承認を必要とする"},
        },
    ),
    (
        "aggregate",
        {
            "type": "noul",
            "instructions": "質問は集計（件数・合計など）を求めているか。",
            "criteria": {"false": "集計を求めていない", "true": "集計を求めている"},
        },
    ),
    (
        "politeness_score",
        {
            "type": "score",
            "instructions": "質問の丁寧さの度合い。",
            "criteria": ["ぞんざい・命令調", "通常の丁寧さ", "非常に丁寧・敬語が多い"],
        },
    ),
    (
        "specific_id",
        {
            "type": "noul",
            "instructions": "質問は特定のID・番号を指しているか。",
            "criteria": {"false": "特定のIDは指していない", "true": "特定のIDを指している"},
        },
    ),
]


def build_request(size):
    questions = {"pick": PICK_QUESTION}
    for key, question in EXTRA_QUESTIONS[: size - 1]:
        questions[key] = question
    return {"state": STATE, "model": "jev-latest", "questions": questions}


def timed_post(url, body, headers):
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers=headers, method="POST")
    start = time.monotonic()
    status = 0
    parsed = None
    err = None
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            status = resp.status
            text = resp.read().decode("utf-8")
            try:
                parsed = json.loads(text)
            except json.JSONDecodeError:
                parsed = None
    except urllib.error.HTTPError as e:
        status = e.code
        err = str(e)
    except Exception as e:  # noqa: BLE001 - report any transport failure as a row
        err = str(e)
    ms = (time.monotonic() - start) * 1000
    return ms, status, parsed, err


def call_jev(body):
    headers = {"Content-Type": "application/json", "Authorization": f"Bearer {API_KEY}"}
    return timed_post(JEV_URL, body, headers)


def call_local_pick(body):
    headers = {"Content-Type": "application/json"}
    return timed_post(LOCAL_PICK_URL, body, headers)


SESSION_COOKIE = None


def sign_in():
    global SESSION_COOKIE
    data = json.dumps({"name": "admin", "password": "dev-only-admin-password"}).encode()
    req = urllib.request.Request(
        LOCAL_SESSION_URL,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        if resp.status != 200:
            raise RuntimeError(f"sign-in failed: {resp.status}")
        set_cookie = resp.headers.get("Set-Cookie")
        if not set_cookie:
            raise RuntimeError("sign-in response carried no Set-Cookie header")
        SESSION_COOKIE = set_cookie.split(";")[0]
    print(f"signed in, cookie: {SESSION_COOKIE.split('=')[0]}=...")


def call_local_plan(body):
    headers = {"Content-Type": "application/json", "Cookie": SESSION_COOKIE}
    return timed_post(LOCAL_PLAN_URL, body, headers)


def percentile(sorted_vals, p):
    if not sorted_vals:
        return None
    n = len(sorted_vals)
    idx = -(-(p * n) // 100) - 1  # ceil(p/100 * n) - 1
    idx = min(n - 1, max(0, idx))
    return sorted_vals[idx]


def stats(ms_list):
    s = sorted(ms_list)
    if not s:
        return {"n": 0, "mean": None, "p50": None, "p90": None, "max": None}
    mean = sum(s) / len(s)
    return {
        "n": len(s),
        "mean": round(mean, 1),
        "p50": round(percentile(s, 50), 1),
        "p90": round(percentile(s, 90), 1),
        "max": round(s[-1], 1),
    }


def append_jsonl(path, row):
    with open(path, "a", encoding="utf-8") as f:
        f.write(json.dumps(row, ensure_ascii=False) + "\n")


def run_part_a():
    print("=== Part A: Jev batch-size curve ===")
    sizes = [1, 2, 4, 8, 13]
    results = {}
    for size in sizes:
        req = build_request(size)
        if size == 13:
            with open(SIZE13_JSON, "w", encoding="utf-8") as f:
                json.dump(req, f, ensure_ascii=False, indent=2)
        print(f"size={size}: warm-up x3")
        for i in range(3):
            _, status, _, err = call_jev(req)
            if status != 200:
                print(f"  warm-up {i} status={status} err={err}")
        print(f"size={size}: measuring x20")
        ms_list, in_tok, out_tok = [], [], []
        for i in range(20):
            ms, status, parsed, err = call_jev(req)
            usage = (parsed or {}).get("usage", {}) if isinstance(parsed, dict) else {}
            row = {
                "phase": "A",
                "size": size,
                "call": i,
                "ms": round(ms, 1),
                "status": status,
                "input_tokens": usage.get("input_tokens"),
                "output_tokens": usage.get("output_tokens"),
                "err": err,
            }
            append_jsonl(BATCH_JSONL, row)
            if status == 200:
                ms_list.append(ms)
                if isinstance(usage.get("input_tokens"), int):
                    in_tok.append(usage["input_tokens"])
                if isinstance(usage.get("output_tokens"), int):
                    out_tok.append(usage["output_tokens"])
            else:
                print(f"  call {i} status={status} err={err}")
        results[size] = {
            "latency": stats(ms_list),
            "input_tokens_mean": round(sum(in_tok) / len(in_tok)) if in_tok else None,
            "output_tokens_mean": round(sum(out_tok) / len(out_tok)) if out_tok else None,
            "calls_ok": len(ms_list),
        }
        print(f"size={size}: {json.dumps(results[size])}")
    return results


def run_at_concurrency(level, total, fn):
    ms_all, rows = [], []
    failures = 0
    wall_start = time.monotonic()
    launched = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(level, 1)) as pool:
        while launched < total:
            batch = min(level, total - launched)
            futures = [pool.submit(fn) for _ in range(batch)]
            for fut in futures:
                r = fut.result()
                rows.append(r)
                if r["status"] == 200:
                    ms_all.append(r["ms"])
                else:
                    failures += 1
            launched += batch
    wall_ms = (time.monotonic() - wall_start) * 1000
    return rows, ms_all, failures, wall_ms


LOCAL_PICK_BODY = {
    "model": "qwen3.5-9b-q8",
    "messages": [
        {
            "role": "system",
            "content": (
                "あなたは社内APIの振り分け役。質問に対して、候補一覧の中から呼ぶべきAPIを1つ選ぶ。\n\n"
                "出力は次の形式の1行だけ。説明もタグも書かない。\nlistInventoryItems certain\n\n"
                "1語目は候補一覧にある operationId をそのまま。2語目は certain か ambiguous。\n"
                "ambiguous は「質問文だけでは候補を1つに決められない」場合。自信の有無ではなく、"
                "質問が足りていない場合。\nambiguous のときも、最も可能性の高い operationId を"
                "必ず1つ挙げること。"
            ),
        },
        {
            "role": "user",
            "content": (
                "質問: 在庫の一覧を見せて\n\n候補:\n"
                "ListInventoryItems\t在庫管理\tList stock items, optionally filtered by status.\n"
                "GetInventoryItem\t在庫管理\tGet one stock item by id.\n"
                "ListAttendanceRecords\t勤怠管理\tList attendance records, optionally filtered by kind.\n"
                "CreateInventoryItem\t在庫管理\tCreate a stock item.\n"
                "GetAttendanceRecord\t勤怠管理\tGet one attendance record by id.\n"
                "CreateAttendanceRecord\t勤怠管理\tCreate an attendance record.\n"
                "list_capabilities\tplatform\t使える操作の一覧を知りたい\n"
                "propose_panel\tplatform\t画面に出したい\n"
                "none\tplatform\tどの候補も質問に合わない（業務と無関係な質問）"
            ),
        },
    ],
    "temperature": 0,
    "max_tokens": 200,
    "chat_template_kwargs": {"enable_thinking": False},
}


def run_part_b():
    print("=== Part B: concurrency ===")
    levels = [1, 2, 4, 8]
    jev_body = build_request(1)

    local_plan_body = None
    try:
        sign_in()
        local_plan_body = {"query": "在庫の一覧を見せて", "turns": [], "thinking": False}
    except Exception as e:  # noqa: BLE001 - degrade gracefully, still report other targets
        print(f"local end-to-end sign-in failed, will skip that target: {e}")

    def jev_call():
        ms, status, _, err = call_jev(jev_body)
        return {"ms": round(ms, 1), "status": status, "err": err}

    def local_pick_call():
        ms, status, _, err = call_local_pick(LOCAL_PICK_BODY)
        return {"ms": round(ms, 1), "status": status, "err": err}

    def local_plan_call():
        ms, status, _, err = call_local_plan(local_plan_body)
        return {"ms": round(ms, 1), "status": status, "err": err}

    targets = [("jev", jev_call), ("local_pick", local_pick_call)]
    if local_plan_body is not None:
        targets.append(("local_end_to_end", local_plan_call))

    summary = {}
    for name, fn in targets:
        summary[name] = {}
        for level in levels:
            print(f"target={name} level={level}: sending 40")
            rows, ms_all, failures, wall_ms = run_at_concurrency(level, 40, fn)
            for row in rows:
                append_jsonl(CONC_JSONL, {"phase": "B", "level": level, "target": name, **row})
            s = stats(ms_all)
            throughput = round(len(rows) / (wall_ms / 1000), 2) if ms_all else 0
            summary[name][level] = {**s, "failures": failures, "throughput_rps": throughput}
            print(f"target={name} level={level}: {json.dumps(summary[name][level])}")
            if failures > 0:
                print(f"target={name}: {failures} failures at level={level}, stopping escalation")
                break
            time.sleep(0.2)
    return summary


def main():
    open(BATCH_JSONL, "w", encoding="utf-8").close()
    open(CONC_JSONL, "w", encoding="utf-8").close()
    a = run_part_a()
    b = run_part_b()
    print("=== SUMMARY A ===")
    print(json.dumps(a, indent=2, ensure_ascii=False))
    print("=== SUMMARY B ===")
    print(json.dumps(b, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
