package graph

import (
	"cmp"
	"shellshift/internal/db"
	"slices"
	"strings"
	"time"
)

type Parent struct {
	Group string     `json:"group"`
	Data  ParentData `json:"data"`
}

type ParentData struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Node struct {
	Group string   `json:"group"`
	Data  NodeData `json:"data"`
}

type NodeData struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt int    `json:"-"`
	Parent    string `json:"parent"`
	Level     int    `json:"level"`
}

type Edge struct {
	Group string   `json:"group"`
	Data  EdgeData `json:"data"`
}

type EdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func buildGraph(chats []db.GetGraphRow) []any {
	if len(chats) == 0 {
		return []any{}
	}
	var graph []any
	tagGroups := groupTags(chats)

	for _, v := range chats {
		// Push nodes to graph slice
		if !slices.ContainsFunc(graph, func(node any) bool {
			return node.(Node).Data.ID == v.ID
		}) {
			var parent string

			for _, tg := range tagGroups {
				if v.Name.Valid && slices.Contains(tg.Tags, v.Name.String) {
					parent = strings.Join(tg.Tags, "-")
				}
			}

			graph = append(graph, Node{
				Group: "nodes",
				Data: NodeData{
					ID:        v.ID,
					Title:     v.Title,
					UpdatedAt: int(v.UpdatedAt),
					Parent:    parent,
				},
			})
		}
	}

	// Set nodes level according to their last updated date
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

	// Push tag groups nodes to graph slice
	for _, tg := range tagGroups {
		graph = append(graph, Parent{
			Group: "nodes",
			Data: ParentData{
				ID:    strings.Join(tg.Tags, "-"),
				Title: "",
			},
		})
	}

	return graph
}

type TagGroup struct {
	Tags    []string
	ChatIds []string
}

func groupTags(chats []db.GetGraphRow) []TagGroup {
	var tagGroups []TagGroup

	for _, chat := range chats {
		if !chat.Name.Valid {
			continue
		}

		id := chat.ID
		tag := chat.Name.String

		tagIdx := slices.IndexFunc(tagGroups, func(t TagGroup) bool {
			return slices.Contains(t.Tags, tag)
		})
		chatIdx := slices.IndexFunc(tagGroups, func(t TagGroup) bool {
			return slices.Contains(t.ChatIds, id)
		})

		switch {
		case tagIdx == -1 && chatIdx == -1:
			tagGroups = append(tagGroups, TagGroup{
				Tags:    []string{tag},
				ChatIds: []string{id},
			})
			// TODO: Figure out this case
		case tagIdx != -1 && chatIdx != -1 && tagIdx != chatIdx:
			tgTag := tagGroups[tagIdx]
			tgChat := tagGroups[chatIdx]

			tgTag.Tags = slices.Compact(slices.Concat(tgTag.Tags, tgChat.Tags))
			tgTag.ChatIds = slices.Compact(slices.Concat(tgTag.ChatIds, tgChat.ChatIds))
			tagGroups[tagIdx] = tgTag

			tagGroups = slices.Delete(tagGroups, chatIdx, chatIdx+1)
			if tagIdx > chatIdx {
				tagIdx--
			}
		case tagIdx != -1:
			tg := tagGroups[tagIdx]
			tg.ChatIds = slices.Compact(append(tg.ChatIds, id))
			tagGroups[tagIdx] = tg
		case chatIdx != -1:
			tg := tagGroups[chatIdx]
			tg.Tags = append(tg.Tags, tag)
			tg.ChatIds = slices.Compact(append(tg.ChatIds, id))
			tagGroups[chatIdx] = tg
		}
	}

	for i := range tagGroups {
		for j := i + 1; j < len(tagGroups); j++ {
			if slicesHaveCommonItem(tagGroups[i].Tags, tagGroups[j].Tags) || slicesHaveCommonItem(tagGroups[i].ChatIds, tagGroups[j].ChatIds) {
				tg := tagGroups[i]
				tg.Tags = slices.Compact(slices.Concat(tg.Tags, tagGroups[j].Tags))
				tg.ChatIds = slices.Compact(slices.Concat(tg.ChatIds, tagGroups[j].ChatIds))
				tagGroups[i] = tg
				tagGroups = slices.Delete(tagGroups, j, j+1)
			}
		}
	}
	return tagGroups
}

func slicesHaveCommonItem[T comparable](s1, s2 []T) bool {
	seen := make(map[T]bool)

	for _, item := range s1 {
		seen[item] = true
	}

	for _, item := range s2 {
		if seen[item] {
			return true
		}
	}

	return false
}
