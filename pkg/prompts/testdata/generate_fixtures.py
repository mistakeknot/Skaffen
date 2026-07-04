#!/usr/bin/env python3
"""Generate golden test fixtures from Python prompts.py for Go parity testing.

Runs Auraken's build_system_prompt across a scenario matrix and records the
exact assembled prompt for each. The Go side (pkg/prompts/parity_test.go and
internal/auraken/parity_test.go) replays the same inputs and requires
byte-identical output.

Scenarios are deterministic: no valid_until on feedback (avoids time.Now
dependence). Instant-mirroring messages are chosen from inputs known not to
trigger the Python .keys()-on-list crash (see pkg/style golden fixtures).

Run from the Sylveste monorepo root:
    python3 os/Skaffen/pkg/prompts/testdata/generate_fixtures.py
"""

import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../../../../apps/Auraken/src"))

from auraken.prompts import build_system_prompt
from auraken.style import compute_observables, update_fingerprint


def make_fingerprint(messages):
    fp = None
    for msg in messages:
        fp = update_fingerprint(fp, compute_observables(msg))
    return json.loads(json.dumps(fp))


# Fingerprint with mixed modes (>= 3 messages so the fingerprint path is used).
FP_GENERAL = make_fingerprint([
    "hey just checking in on the project",
    "the root cause is a constraint in the framework",
    "cool thanks",
    "what time tomorrow? i'm free at 3pm",
    "yeah totally",
])

# Fingerprint dominated by emotional/intimate messages.
FP_EMOTIONAL = make_fingerprint([
    "i feel so overwhelmed right now, i can't handle this",
    "i'm struggling with anxiety, i'm afraid of what happens next",
    "i love you baby, sleep well, sweet dreams",
    "i'm really so stressed honestly, i literally can't deal",
])

LENS_FULL = {
    "name": "Sunk Cost",
    "description": "Past investment should not drive future decisions.",
    "when_to_apply": "When someone clings to a path because of what they already spent.",
    "how_to_apply": "Ask about future value, not past cost.",
    "forces": "Loss aversion; commitment escalation",
    "questions": "  - If you were starting fresh today, would you choose this?\n  - What would walking away actually cost you going forward?",
}

LENS_BARE = {
    "name": "Principal-Agent",
    "description": "Interests of the actor and the owner diverge.",
    "when_to_apply": "When incentives look misaligned.",
    "how_to_apply": "Identify who bears the cost of each decision.",
}


def numbered_lenses(n):
    return [
        {
            "name": f"Lens {i}",
            "description": f"Description {i}.",
            "when_to_apply": f"Condition {i}.",
            "how_to_apply": f"Application {i}.",
        }
        for i in range(1, n + 1)
    ]


SCENARIOS = [
    {
        "name": "new_user_first_contact",
        "input": {
            "is_new_user": True,
            "interaction_count": 0,
            "current_message": "hey how are you",
        },
    },
    {
        "name": "bare_minimum",
        "input": {},
    },
    {
        "name": "second_exchange",
        "input": {
            "interaction_count": 1,
            "recent_turns": [
                {"role": "user", "content": "i keep going back and forth on quitting my job"},
                {"role": "assistant", "content": "what's pulling you in each direction?"},
            ],
            "current_message": "i'm really stressed",
        },
    },
    {
        "name": "third_exchange_reframe",
        "input": {
            "interaction_count": 2,
            "working_profile": "Engineer weighing a career change; values autonomy but fears instability.",
            "explored_domains": ["goals"],
            "known_entities": [{"domain": "goals", "value": "wants to start a company"}],
            "recent_turns": [
                {"role": "user", "content": "i keep going back and forth on quitting my job"},
                {"role": "assistant", "content": "what's pulling you in each direction?"},
                {"role": "user", "content": "money on one side, sanity on the other"},
                {"role": "assistant", "content": "which one runs out first?"},
            ],
            "current_message": "hey how are you",
        },
    },
    {
        "name": "echo_at_five_full_context",
        "input": {
            "interaction_count": 5,
            "working_profile": "Product-minded engineer. Optimizes for stability while claiming to want growth. Defers decisions when stakes involve other people.",
            "explored_domains": ["goals", "values", "constraints"],
            "known_entities": [
                {"domain": "goals", "value": "launch a startup"},
                {"domain": "values", "value": "autonomy"},
                {"domain": "constraints", "value": "mortgage payments"},
                {"domain": "patterns", "value": "defers to spouse on money"},
            ],
            "relevant_lenses": [LENS_FULL, LENS_BARE],
            "feedback_entities": [
                {"value": "Don't use exclamation marks", "confidence": 0.9},
                {"value": "Be more direct with me", "confidence": 0.85},
            ],
            "style_fingerprint": FP_GENERAL,
            "recent_turns": [
                {"role": "user", "content": "so the reorg got announced today"},
                {"role": "assistant", "content": "where did you land?"},
                {"role": "user", "content": "same team, new manager. third one this year"},
                {"role": "assistant", "content": "three managers in a year — what stays constant?"},
                {"role": "user", "content": "honestly just me and the codebase"},
                {"role": "assistant", "content": "maybe that's the real org chart."},
            ],
            "current_message": "should i even bother building relationships with this one",
        },
    },
    {
        "name": "seven_followup_reflect",
        "input": {
            "interaction_count": 7,
            "working_profile": "Founder in year two. Pattern of over-committing then renegotiating.",
            "explored_domains": [
                "goals", "constraints", "values", "priorities",
                "patterns", "decisions", "skills", "relationships",
            ],
            "known_entities": [
                {"domain": "goals", "value": "raise a seed round"},
                {"domain": "constraints", "value": "six months of runway"},
                {"domain": "values", "value": "craft over speed"},
                {"domain": "priorities", "value": "hiring a designer"},
                {"domain": "patterns", "value": "overcommits then renegotiates"},
                {"domain": "decisions", "value": "chose bootstrapping first"},
            ],
            "style_fingerprint": FP_EMOTIONAL,
            "recent_turns": [
                {"role": "user", "content": "i feel so overwhelmed right now, i can't handle this"},
                {"role": "assistant", "content": "what's the loudest thing on the list?"},
                {"role": "user", "content": "the investor update. i keep rewriting it"},
            ],
            "current_message": "maybe i should just skip this month's update",
        },
    },
    {
        "name": "pattern_hunting_at_four",
        "input": {
            "interaction_count": 4,
            "working_profile": "Early profile. Two data points so far.",
            "known_entities": [
                {"domain": "goals", "value": "finish the thesis"},
                {"domain": "constraints", "value": "teaching load"},
            ],
            "recent_turns": [
                {"role": "user", "content": "advisor moved the defense date again"},
                {"role": "assistant", "content": "how many times is that now?"},
            ],
            "current_message": "hey how are you",
        },
    },
    {
        "name": "lens_limit_six",
        "input": {
            "interaction_count": 3,
            "working_profile": "Knows their way around a spreadsheet.",
            "relevant_lenses": numbered_lenses(6),
        },
    },
    {
        "name": "feedback_sanitize_and_unicode",
        "input": {
            "interaction_count": 3,
            "working_profile": "Test profile.",
            "feedback_entities": [
                {"value": "# Héader\n```codé```\nkeep this — payload", "confidence": 0.7},
                {"value": "é" * 250, "confidence": 1.0},
                {"value": "###", "confidence": 0.5},
            ],
        },
    },
    {
        "name": "emotional_fingerprint_mode_detection",
        "input": {
            "interaction_count": 9,
            "working_profile": "Profile present.",
            "style_fingerprint": FP_EMOTIONAL,
            "recent_turns": [
                {"role": "user", "content": "i'm struggling with anxiety, i'm afraid of what happens next"},
                {"role": "assistant", "content": "what's the fear pointing at?"},
                {"role": "user", "content": "i feel so overwhelmed right now, i can't handle this"},
            ],
            "current_message": "everything is falling apart",
        },
    },
]


def generate():
    out = {"version": 1, "generator": "python3 auraken.prompts", "scenarios": []}

    for sc in SCENARIOS:
        inp = sc["input"]
        kwargs = dict(
            working_profile=inp.get("working_profile"),
            recent_turns=inp.get("recent_turns"),
            style_fingerprint=inp.get("style_fingerprint"),
            explored_domains=set(inp["explored_domains"]) if inp.get("explored_domains") is not None else None,
            known_entities=inp.get("known_entities"),
            interaction_count=inp.get("interaction_count", 0),
            relevant_lenses=inp.get("relevant_lenses"),
            feedback_entities=inp.get("feedback_entities"),
            is_new_user=inp.get("is_new_user", False),
            current_message=inp.get("current_message"),
        )
        prompt = build_system_prompt(**kwargs)
        out["scenarios"].append({"name": sc["name"], "input": inp, "prompt": prompt})

    out_path = os.path.join(os.path.dirname(__file__), "golden_fixtures.json")
    with open(out_path, "w") as f:
        json.dump(out, f, indent=2, ensure_ascii=False)

    print(f"Generated {len(out['scenarios'])} scenarios to {out_path}")
    for sc in out["scenarios"]:
        print(f"  {sc['name']}: {len(sc['prompt'])} chars")


if __name__ == "__main__":
    generate()
