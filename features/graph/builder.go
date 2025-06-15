package graph

import (
	"fmt"
	"shellshift/internal/db"
	"slices"
)

type NodeData struct {
	ID string `json:"id"`
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
					ID: v.ID,
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
