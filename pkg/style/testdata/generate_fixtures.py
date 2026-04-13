#!/usr/bin/env python3
"""Generate golden test fixtures from Python style.py for Go parity testing.

Run from the Sylveste monorepo root:
    python3 os/Skaffen/pkg/style/testdata/generate_fixtures.py
"""

import json
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../../../../apps/Auraken/src"))

from auraken.style import (
    compute_observables,
    update_fingerprint,
    classify_mode,
    build_mirroring_instructions,
    build_instant_mirroring,
    detect_current_mode,
)

# 20 test messages covering all modes and edge cases.
# Messages 4-6 bracket the alpha transition (n=4→5→6).
MESSAGES = [
    # 1: emotional
    "i feel so overwhelmed right now, i can't handle this",
    # 2: analytical
    "the root cause is a constraint in the framework, however the evidence suggests a trade-off",
    # 3: playful
    "haha omg that's so wild lol",
    # 4: intimate (n=3 for global, alpha still adaptive)
    "i love you baby, sleep well, sweet dreams",
    # 5: logistics (n=4 for global → alpha = 1/(4+1) = 0.2)
    "what time tomorrow? i'm free at 3pm, sounds good",
    # 6: update (n=5 for global → alpha switches to 0.3)
    "hey just got home, work was long, day was exhausting",
    # 7: general (no mode patterns)
    "cool thanks",
    # 8: emotional again (builds per-mode profile)
    "i'm struggling with anxiety, i'm afraid of what happens next",
    # 9: all lowercase, no punctuation, short
    "yeah totally",
    # 10: multi-sentence with exclamation
    "This is amazing! I can't believe it. Wow!",
    # 11: playful with emoji
    "haha \U0001F602 that's cursed lmao",
    # 12: analytical with hedges
    "i think maybe the assumption here is probably wrong, although i'm not sure",
    # 13: logistics with time
    "when are we meeting? next week works for me, confirm the reservation",
    # 14: update short
    "hi how's your day",
    # 15: emotional with intensifiers
    "i'm really so stressed honestly, i literally can't deal",
    # 16: tie-breaking boundary — both playful and intimate score
    "haha baby that's wild",
    # 17: all caps
    "THIS IS IMPORTANT",
    # 18: contraction-heavy
    "i'm don't think we're they're it's",
    # 19: empty-ish (only spaces after strip → tests edge)
    "ok",
    # 20: laughter with ahaha (tests duplicate label behavior)
    "haha ahaha that was funny",
]


def generate():
    fingerprint = None
    steps = []

    for i, msg in enumerate(MESSAGES):
        obs = compute_observables(msg)
        mode = classify_mode(msg)
        fingerprint = update_fingerprint(fingerprint, obs)

        # Capture fingerprint state after each message
        steps.append({
            "index": i,
            "message": msg,
            "observables": obs,
            "mode": mode,
            "fingerprint_after": json.loads(json.dumps(fingerprint)),
        })

    # Build mirroring instructions from final fingerprint
    mirroring = {}
    for mode_name in ["general", "emotional", "analytical", "playful",
                       "intimate", "logistics", "update"]:
        mirroring[mode_name] = build_mirroring_instructions(fingerprint, mode_name)

    # Build instant mirroring for a few messages
    instant = {}
    for msg in ["hey how are you", "haha that's so funny", "i'm really stressed"]:
        try:
            result = build_instant_mirroring(msg)
        except AttributeError:
            # Python bug: .keys() on a list — mark as crashed
            result = "PYTHON_BUG_SKIPPED"
        instant[msg] = result

    # Detect current mode
    detect_results = {
        "last_5_all": detect_current_mode(MESSAGES, 5),
        "last_3": detect_current_mode(MESSAGES[-3:], 5),
        "empty": detect_current_mode([], 5),
    }

    fixture = {
        "version": 1,
        "generator": "python3 auraken.style",
        "message_count": len(MESSAGES),
        "steps": steps,
        "final_mirroring": mirroring,
        "instant_mirroring": instant,
        "detect_mode": detect_results,
    }

    out_path = os.path.join(os.path.dirname(__file__), "golden_fixtures.json")
    with open(out_path, "w") as f:
        json.dump(fixture, f, indent=2, ensure_ascii=False)

    print(f"Generated {len(steps)} steps to {out_path}")
    print(f"Final message_count: {fingerprint['message_count']}")

    # Verify instant mirroring bug
    bug_count = sum(1 for v in instant.values() if v == "PYTHON_BUG_SKIPPED")
    if bug_count > 0:
        print(f"Python bug confirmed: {bug_count} instant mirroring calls crashed")


if __name__ == "__main__":
    generate()
