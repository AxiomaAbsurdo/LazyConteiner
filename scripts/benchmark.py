#!/usr/bin/env python3
"""Measure read-only dashboard requests; print aggregate metrics only."""
import argparse
import json
import statistics
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--url', default='http://127.0.0.1:17878/api/dashboard')
    parser.add_argument('--requests', type=int, default=20)
    args = parser.parse_args()
    if not 2 <= args.requests <= 100:
        parser.error('--requests must be between 2 and 100')
    durations, sizes = [], []
    for _ in range(args.requests):
        started = time.perf_counter()
        with urllib.request.urlopen(args.url, timeout=30) as response:
            body = response.read()
        durations.append((time.perf_counter() - started) * 1000)
        sizes.append(len(body))
        payload = json.loads(body)
        if not isinstance(payload.get('stacks'), list):
            raise ValueError('Response is not a dashboard')
    ordered = sorted(durations)
    print(json.dumps({
        'requests': len(durations), 'mode': payload.get('mode'),
        'containers': payload.get('totals', {}).get('containers'),
        'cold_ms': round(durations[0], 2),
        'median_ms': round(statistics.median(durations), 2),
        'p95_ms': round(ordered[int((len(ordered) - 1) * 0.95)], 2),
        'max_ms': round(max(durations), 2),
        'mean_payload_bytes': round(statistics.mean(sizes)),
    }, indent=2))


if __name__ == '__main__':
    main()
