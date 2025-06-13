package graph

import (
	"cmp"
	"fmt"
	"shellshift/internal/db"
	"slices"
	"time"
)

type NodeData struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt int    `json:"-"`
	Level     int    `json:"level"`
}

type EdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Tag    string `json:"tag"`
}

type Node struct {
	Group string   `json:"group"`
	Data  NodeData `json:"data"`
}

type Edge struct {
	Group string   `json:"group"`
	Data  EdgeData `json:"data"`
}

func buildGraph(chats []db.GetGraphRow) []any {
	m := make(map[string][]string)

	var graph []any

	for _, v := range chats {
		// Push nodes to graph slice
		if !slices.ContainsFunc(graph, func(node any) bool {
			return node.(Node).Data.ID == v.ID
		}) {
			graph = append(graph, Node{
				Group: "nodes",
				Data: NodeData{
					ID:        v.ID,
					Title:     v.Title,
					UpdatedAt: int(v.UpdatedAt),
				},
			})
		}

		if !v.Name.Valid {
			continue
		}
		// Build [tag]:[]ids map
		tag := v.Name.String

		if s, ok := m[tag]; !ok {
			m[tag] = []string{v.ID}
		} else {
			m[tag] = append(s, v.ID)
		}
	}

	// Set nodes level according to
	slices.SortFunc(graph, func(a, b any) int {
		return cmp.Compare(b.(Node).Data.UpdatedAt, a.(Node).Data.UpdatedAt)
	})

	lastUpdated := time.Unix(int64(graph[0].(Node).Data.UpdatedAt), 0)
	t1 := time.Date(lastUpdated.Year(), lastUpdated.Month(), lastUpdated.Day(), 0, 0, 0, 0, time.UTC)

	for i := 1; i < len(graph); i++ {
		currentUpdated := time.Unix(int64(graph[i].(Node).Data.UpdatedAt), 0)
		t2 := time.Date(currentUpdated.Year(), currentUpdated.Month(), currentUpdated.Day(), 0, 0, 0, 0, time.UTC)

		dayDiff := int(t1.Sub(t2).Hours() / 24)

		node := graph[i].(Node)
		node.Data.Level = dayDiff + 1
		graph[i] = node
	}


	// Collect edges
	for tag, ids := range m {
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				graph = append(graph, Edge{
					Group: "edges",
					Data: EdgeData{
						ID:     fmt.Sprintf("%s:%s", ids[i], ids[j]),
						Source: ids[i],
						Target: ids[j],
						Tag:    tag,
					},
				})
			}
		}
	}

	return graph
}
