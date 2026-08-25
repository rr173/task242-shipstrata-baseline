// Package strata 实现层位偏序求解：基于「上覆单元晚于下伏单元」构建有向图，
// 检测不可能循环（矛盾）并识别疑似后期侵扰层 / 测绘误连。
package strata

import (
	"fmt"
	"sort"
	"strings"

	"task242-shipstrata/internal/model"
)

type edge struct {
	id, from, to, rel string
	seq               int
}

// SolveResult 求解结果。
type SolveResult struct {
	Status         map[string]string // contactID -> 新状态（conflict 或关系名）
	Cycles         [][]string        // 每个不可能循环的去重单元序列
	Contradictions []ContradictionInput
	Intrusions     []IntrusionInput
}

// ContradictionInput 待写入的矛盾记录。
type ContradictionInput struct {
	CycleUnits       []string
	InvolvedContacts []string
}

// IntrusionInput 待写入的侵扰候选。
type IntrusionInput struct {
	UnitID             string
	Reason             string
	SupportingContacts []string
}

// Solve 对所有「非排除」接触关系构建偏序图并检测循环。
// 每条接触关系构成有向边 from→to（上覆 / 切割 均表示 from 晚于 to）。
func Solve(contacts []model.Contact) SolveResult {
	res := SolveResult{Status: map[string]string{}}
	adj := map[string][]string{}
	var edges []edge
	for _, c := range contacts {
		if !model.ContactParticipates(c.Status) {
			continue
		}
		adj[c.FromUnitID] = append(adj[c.FromUnitID], c.ToUnitID)
		edges = append(edges, edge{c.ID, c.FromUnitID, c.ToUnitID, c.Relation, c.SurveySeq})
	}

	seenCycles := map[string]bool{}
	for _, e := range edges {
		if reachable(e.to, e.from, adj, e) {
			res.Status[e.id] = model.ContactStatusConflict
			path := pathTo(e.to, e.from, adj, e)
			if path == nil {
				path = []string{e.to, e.from}
			}
			cycleNodes := dedup(path)
			key := sortedKey(cycleNodes)
			if !seenCycles[key] {
				seenCycles[key] = true
				inCycle := cycleContacts(cycleNodes, edges)
				res.Cycles = append(res.Cycles, cycleNodes)
				res.Contradictions = append(res.Contradictions, ContradictionInput{
					CycleUnits:       cycleNodes,
					InvolvedContacts: inCycle,
				})
				suspect := mostConnected(cycleNodes, edges)
				res.Intrusions = append(res.Intrusions, IntrusionInput{
					UnitID:             suspect,
					Reason:             fmt.Sprintf("单元 %s 处于不可能循环 %v 中，疑似后期侵扰层或测绘误连", suspect, cycleNodes),
					SupportingContacts: inCycle,
				})
			}
		} else {
			res.Status[e.id] = model.ContactStatusPending
		}
	}
	return res
}

// reachable 判断从 start 是否可达 target（跳过 skip 边）。
func reachable(start, target string, adj map[string][]string, skip edge) bool {
	visited := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == target {
			return true
		}
		if visited[n] {
			continue
		}
		visited[n] = true
		for _, m := range adj[n] {
			if n == skip.from && m == skip.to {
				continue
			}
			if !visited[m] {
				stack = append(stack, m)
			}
		}
	}
	return false
}

// pathTo 返回从 start 到 target 的一条路径（含两端），跳过 skip 边。
func pathTo(start, target string, adj map[string][]string, skip edge) []string {
	var dfs func(n string, path []string, visited map[string]bool) []string
	dfs = func(n string, path []string, visited map[string]bool) []string {
		if n == target {
			return append(append([]string{}, path...), n)
		}
		visited[n] = true
		for _, m := range adj[n] {
			if n == skip.from && m == skip.to {
				continue
			}
			if visited[m] {
				continue
			}
			if r := dfs(m, append(path, n), visited); r != nil {
				return r
			}
		}
		return nil
	}
	return dfs(start, nil, map[string]bool{})
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortedKey(nodes []string) string {
	cp := append([]string{}, nodes...)
	sort.Strings(cp)
	return strings.Join(cp, ",")
}

func cycleContacts(nodes []string, edges []edge) []string {
	set := map[string]bool{}
	for _, n := range nodes {
		set[n] = true
	}
	var out []string
	for _, e := range edges {
		if set[e.from] && set[e.to] {
			out = append(out, e.id)
		}
	}
	return out
}

// mostConnected 返回环内连接度最高的单元（最可能为侵扰层）。
func mostConnected(nodes []string, edges []edge) string {
	set := map[string]bool{}
	for _, n := range nodes {
		set[n] = true
	}
	deg := map[string]int{}
	for _, e := range edges {
		if set[e.from] && set[e.to] {
			deg[e.from]++
			deg[e.to]++
		}
	}
	best := ""
	bestDeg := -1
	for _, n := range nodes {
		if deg[n] > bestDeg {
			bestDeg = deg[n]
			best = n
		}
	}
	return best
}
