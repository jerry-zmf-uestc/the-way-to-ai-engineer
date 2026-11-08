"""Original bounded beam search over immutable state IDs; no model or external actions."""
from __future__ import annotations
from dataclasses import dataclass
import argparse
import math


@dataclass(frozen=True)
class Limits:
    width: int = 2
    depth: int = 3
    nodes: int = 16
    per_parent: int = 4


@dataclass(frozen=True)
class Result:
    path: tuple | None
    reason: str
    expanded: int
    generated: int
    evaluated: int


def search(initial, propose, evaluate, verify, limits=Limits(), cancelled=lambda: False, goal_policy='retained'):
    if goal_policy not in ('retained', 'generated'):
        raise ValueError('invalid goal policy')
    for value, maximum in ((limits.width, 8), (limits.depth, 8), (limits.nodes, 64), (limits.per_parent, 8)):
        if type(value) is not int or not 0 < value <= maximum:
            raise ValueError('invalid limits')
    if not isinstance(initial, str) or not initial or len(initial) > 128:
        raise ValueError('invalid initial state')
    expanded = generated = evaluated = 0

    def result(reason, path=None):
        return Result(path, reason, expanded, generated, evaluated)

    def goal(path):
        if cancelled():
            return None
        solved = verify(path)
        if cancelled():
            return None
        if type(solved) is not bool:
            raise ValueError('invalid goal result')
        return solved

    solved = goal((initial,))
    if solved is None:
        return result('cancelled')
    if solved:
        return result('solved', (initial,))
    frontier = [(initial,)]
    for _ in range(limits.depth):
        children = []
        for path in frontier:
            if cancelled():
                return result('cancelled')
            expanded += 1
            candidates = propose(path)
            if cancelled():
                return result('cancelled')
            if not isinstance(candidates, list) or len(candidates) > limits.per_parent:
                raise ValueError('candidate bound')
            if any(not isinstance(x, str) or not x or len(x) > 128 for x in candidates) or len(set(candidates)) != len(candidates):
                raise ValueError('invalid candidates')
            candidates = tuple(candidates)
            for candidate in candidates:
                if generated >= limits.nodes:
                    return result('node_budget')
                generated += 1
                if candidate in path:
                    continue
                child = path + (candidate,)
                if goal_policy == 'generated':
                    solved = goal(child)
                    if solved is None:
                        return result('cancelled')
                    if solved:
                        return result('solved', child)
                score = evaluate(child)
                evaluated += 1
                if cancelled():
                    return result('cancelled')
                if type(score) not in (int, float) or (type(score) is float and not math.isfinite(score)):
                    raise ValueError('invalid score')
                children.append((score, child))
        frontier = [path for _, path in sorted(children, key=lambda item: (-item[0], item[1]))[:limits.width]]
        if not frontier:
            return result('exhausted')
        if goal_policy == 'retained':
            for path in frontier:
                solved = goal(path)
                if solved is None:
                    return result('cancelled')
                if solved:
                    return result('solved', path)
    return result('depth_budget')


def sample(limits):
    links = {'root': ['dead', 'bridge'], 'bridge': ['answer']}
    # Deliberately misleading heuristic: the dead branch outranks the useful bridge.
    return search('root', lambda path: links.get(path[-1], []),
                  lambda path: {'dead': 9, 'bridge': 3, 'answer': 10}[path[-1]],
                  lambda path: path[-1] == 'answer', limits)


def self_check():
    greedy = sample(Limits(width=1))
    wider = sample(Limits(width=2))
    bounded = sample(Limits(width=2, nodes=2))
    assert greedy.reason == 'exhausted' and greedy.path is None and greedy.generated == 2
    assert wider.reason == 'solved' and wider.path == ('root', 'bridge', 'answer')
    assert wider.generated == wider.evaluated == 3
    assert bounded.reason == 'node_budget' and bounded.generated == bounded.evaluated == 2
    assert sample(Limits(depth=1)).reason == 'depth_budget'
    cycle = search('root', lambda path: ['root'], lambda path: 0, lambda path: False)
    assert cycle.reason == 'exhausted' and cycle.generated == 1 and cycle.evaluated == 0
    shared = ['dead', 'bridge']

    def mutating_score(path):
        shared.clear()
        return {'dead': 9, 'bridge': 3, 'answer': 10}[path[-1]]

    isolated = search('root', lambda path: shared if path[-1] == 'root' else ['answer'] if path[-1] == 'bridge' else [],
                      mutating_score, lambda path: path[-1] == 'answer')
    assert isolated.reason == 'solved' and isolated.path == ('root', 'bridge', 'answer')
    stopped = False

    def late_score(path):
        nonlocal stopped
        stopped = True
        return 100

    late = search('root', lambda path: ['answer'], late_score,
                  lambda path: path[-1] == 'answer', cancelled=lambda: stopped)
    assert late.reason == 'cancelled' and late.path is None and late.evaluated == 1
    large = search('root', lambda path: ['answer'], lambda path: 10 ** 400,
                   lambda path: path[-1] == 'answer')
    assert large.reason == 'solved'
    for bad_score in (float('nan'), float('inf'), True):
        try:
            search('root', lambda path: ['answer'], lambda path: bad_score, lambda path: False)
        except ValueError:
            pass
        else:
            raise AssertionError('invalid score accepted')
    for bad_candidates in (['same', 'same'], ['x'] * 5):
        try:
            search('root', lambda path: bad_candidates, lambda path: 0, lambda path: False)
        except ValueError:
            pass
        else:
            raise AssertionError('invalid candidates accepted')
    try:
        sample(Limits(width=0))
    except ValueError:
        pass
    else:
        raise AssertionError('invalid width accepted')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--self-check', action='store_true')
    args = parser.parse_args()
    if args.self_check:
        self_check()
        print('bounded search: budget, pruning, cycles, cancel and shape checks passed')
    else:
        for limits in (Limits(width=1), Limits(width=2), Limits(width=2, nodes=2)):
            print(sample(limits))
