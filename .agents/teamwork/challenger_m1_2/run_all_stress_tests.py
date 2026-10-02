"""
Master Stress Test Runner for M1 IPC & Process Lifecycle.
=========================================================
Runs all 4 stress suites and aggregates metrics:
1. stress_socket_disconnects.py
2. stress_concurrent_clients.py
3. stress_malformed_and_overruns.py
4. stress_process_and_locks.py
"""

import subprocess
import sys
import time
from pathlib import Path

SUITES = [
    "stress_socket_disconnects.py",
    "stress_concurrent_clients.py",
    "stress_malformed_and_overruns.py",
    "stress_process_and_locks.py",
]

def run_suite(suite_name: str) -> tuple[bool, float, str]:
    script_path = Path(__file__).parent / suite_name
    print(f"\n{'='*70}\nRUNNING SUITE: {suite_name}\n{'='*70}")
    t0 = time.perf_counter()
    proc = subprocess.run(
        [sys.executable, str(script_path)],
        capture_output=True,
        text=True,
    )
    elapsed = time.perf_counter() - t0
    output = proc.stdout + ("\nSTDERR:\n" + proc.stderr if proc.stderr else "")
    print(output)
    passed = (proc.returncode == 0)
    return passed, elapsed, output

def main():
    total_start = time.perf_counter()
    results = {}
    print("=" * 70)
    print("ANTIGRAVITY SWISS KNIFE — M1 IPC & PROCESS LIFECYCLE STRESS HARNESS")
    print("=" * 70)

    for suite in SUITES:
        passed, elapsed, out = run_suite(suite)
        results[suite] = {"passed": passed, "elapsed": elapsed, "output": out}
        if not passed:
            print(f"FAILED: {suite} (exited with non-zero)")

    total_elapsed = time.perf_counter() - total_start
    print("\n" + "=" * 70)
    print("STRESS TEST SUMMARY")
    print("=" * 70)
    all_passed = True
    for suite, res in results.items():
        status = "PASSED" if res["passed"] else "FAILED"
        print(f"[{status}] {suite:<35} in {res['elapsed']:.2f}s")
        if not res["passed"]:
            all_passed = False

    print(f"\nTotal execution time: {total_elapsed:.2f}s")
    if all_passed:
        print(">>> ALL ADVERSARIAL STRESS SUITES PASSED EMPIRICALLY! <<<")
        sys.exit(0)
    else:
        print(">>> SOME STRESS SUITES FAILED! <<<")
        sys.exit(1)

if __name__ == "__main__":
    main()
