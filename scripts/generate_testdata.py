#!/usr/bin/env python3
"""Generate historical test data for Prism across multiple time periods."""

import json
import random
import time
import urllib.request
import uuid

INGEST_URL = "http://localhost:24318/api/v1/spans"

SERVICES = [
    {"name": "api-gateway", "ops": ["GET /api/orders", "GET /api/users", "POST /api/orders", "GET /api/products"]},
    {"name": "order-service", "ops": ["getOrder", "createOrder", "listOrders", "cancelOrder"]},
    {"name": "user-service", "ops": ["getUser", "updateUser", "authenticate"]},
    {"name": "payment-service", "ops": ["processPayment", "refund", "getBalance"]},
    {"name": "notification-service", "ops": ["sendEmail", "sendSMS", "pushNotify"]},
    {"name": "inventory-service", "ops": ["checkStock", "reserveItem", "releaseItem"]},
]

CALL_CHAINS = [
    # gateway -> order -> user + payment + inventory
    [
        ("api-gateway", "GET /api/orders", "SERVER"),
        ("order-service", "getOrder", "SERVER"),
        ("user-service", "getUser", "SERVER"),
    ],
    [
        ("api-gateway", "POST /api/orders", "SERVER"),
        ("order-service", "createOrder", "SERVER"),
        ("payment-service", "processPayment", "SERVER"),
        ("inventory-service", "checkStock", "SERVER"),
        ("notification-service", "sendEmail", "SERVER"),
    ],
    [
        ("api-gateway", "GET /api/users", "SERVER"),
        ("user-service", "authenticate", "SERVER"),
    ],
    [
        ("api-gateway", "GET /api/products", "SERVER"),
        ("inventory-service", "checkStock", "SERVER"),
    ],
    [
        ("api-gateway", "POST /api/orders", "SERVER"),
        ("order-service", "cancelOrder", "SERVER"),
        ("payment-service", "refund", "SERVER"),
        ("inventory-service", "releaseItem", "SERVER"),
        ("notification-service", "sendSMS", "SERVER"),
    ],
]

def gen_id(n=16):
    return uuid.uuid4().hex[:n*2]

def send_spans(spans):
    data = json.dumps(spans).encode()
    req = urllib.request.Request(INGEST_URL, data=data, headers={"Content-Type": "application/json"})
    try:
        resp = urllib.request.urlopen(req)
        return json.loads(resp.read())
    except Exception as e:
        print(f"  Error sending {len(spans)} spans: {e}")
        return None

def generate_trace(base_time_us, error_chance=0.05, slow_chance=0.1):
    chain = random.choice(CALL_CHAINS)
    trace_id = gen_id(16)
    spans = []
    parent_id = ""

    base_latency = random.uniform(5000, 30000)  # 5-30ms in us
    is_slow = random.random() < slow_chance
    if is_slow:
        base_latency *= random.uniform(3, 10)

    current_time = base_time_us

    for i, (service, operation, kind) in enumerate(chain):
        span_id = gen_id(8)

        # Duration varies by position in chain and randomness
        duration = base_latency / (i + 1) * random.uniform(0.5, 1.5)
        duration = max(duration, 500)  # at least 0.5ms

        is_error = random.random() < error_chance
        status = "error" if is_error else "ok"

        tags = {"http.method": "GET"}
        if "POST" in operation:
            tags["http.method"] = "POST"
        if is_error:
            tags["error.type"] = random.choice(["timeout", "connection_refused", "internal_error", "not_found"])
            tags["http.status_code"] = random.choice(["500", "502", "503", "404"])
        else:
            tags["http.status_code"] = "200"
        tags["peer.service"] = chain[i+1][0] if i+1 < len(chain) else ""

        span = {
            "trace_id": trace_id,
            "span_id": span_id,
            "service": service,
            "operation": operation,
            "kind": kind,
            "start_us": int(current_time),
            "duration_us": int(duration),
            "status": status,
            "tags": {k: v for k, v in tags.items() if v},
        }
        if parent_id:
            span["parent_span_id"] = parent_id

        spans.append(span)
        parent_id = span_id
        current_time += random.uniform(100, 2000)  # small delay between spans

    return spans

def main():
    now_us = int(time.time() * 1_000_000)
    all_spans = []

    periods = [
        # (hours_ago, traces_count, error_rate, description)
        (0.0,  20, 0.03, "just now"),
        (0.25, 15, 0.05, "15 min ago"),
        (0.5,  15, 0.04, "30 min ago"),
        (1.0,  20, 0.08, "1 hour ago"),
        (2.0,  25, 0.06, "2 hours ago"),
        (3.0,  20, 0.10, "3 hours ago - high errors"),
        (4.0,  15, 0.03, "4 hours ago"),
        (6.0,  25, 0.05, "6 hours ago"),
        (8.0,  20, 0.15, "8 hours ago - incident"),
        (12.0, 30, 0.04, "12 hours ago"),
        (18.0, 20, 0.03, "18 hours ago"),
        (24.0, 25, 0.05, "24 hours ago"),
    ]

    total_traces = 0
    for hours_ago, count, error_rate, desc in periods:
        period_spans = []
        base = now_us - int(hours_ago * 3600 * 1_000_000)

        for i in range(count):
            # Spread traces within a ~30min window
            offset = random.uniform(-15 * 60, 15 * 60) * 1_000_000
            trace_time = base + offset
            slow_chance = 0.15 if "incident" in desc else 0.08
            spans = generate_trace(trace_time, error_chance=error_rate, slow_chance=slow_chance)
            period_spans.extend(spans)

        total_traces += count
        all_spans.extend(period_spans)
        print(f"  {desc}: {count} traces, {len(period_spans)} spans")

    # Send in batches of 200
    batch_size = 200
    sent = 0
    for i in range(0, len(all_spans), batch_size):
        batch = all_spans[i:i+batch_size]
        result = send_spans(batch)
        if result:
            sent += result.get("accepted", 0)

    print(f"\nDone! {total_traces} traces, {sent}/{len(all_spans)} spans sent.")

if __name__ == "__main__":
    main()
