package desktop

import (
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

func exportMetadata(content string) (*yaml.Node, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimPrefix(content, "\ufeff"), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return nil, nil
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		if line != "---" && line != "..." {
			continue
		}
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &node); err != nil {
			return nil, fmt.Errorf("YAML 前言无效：%w", err)
		}
		if len(node.Content) == 0 {
			return nil, nil
		}
		root := node.Content[0]
		if root.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("YAML 前言必须是键值映射")
		}
		return root, nil
	}
	return nil, fmt.Errorf("YAML 前言缺少结束分隔符")
}

func metadataLookup(node *yaml.Node, path []string, depth int) (*yaml.Node, error) {
	visits := 0
	return metadataLookupBounded(node, path, depth, &visits)
}

func metadataLookupBounded(node *yaml.Node, path []string, depth int, visits *int) (*yaml.Node, error) {
	*visits++
	if *visits > 10000 {
		return nil, fmt.Errorf("YAML 变量查找展开过大")
	}
	if node == nil {
		return nil, nil
	}
	if depth > 64 {
		return nil, fmt.Errorf("YAML 变量引用层级过深或存在循环")
	}
	if node.Kind == yaml.AliasNode {
		return metadataLookupBounded(node.Alias, path, depth+1, visits)
	}
	if len(path) == 0 {
		return node, nil
	}
	if node.Kind == yaml.MappingNode {
		var found *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == path[0] {
				if found != nil {
					return nil, fmt.Errorf("YAML 前言重复字段：%s", path[0])
				}
				found = node.Content[i+1]
			}
		}
		if found != nil {
			return metadataLookupBounded(found, path[1:], depth+1, visits)
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Tag == "!!merge" {
				merge := node.Content[i+1]
				if merge.Kind == yaml.SequenceNode {
					for _, candidate := range merge.Content {
						value, err := metadataLookupBounded(candidate, path, depth+1, visits)
						if err != nil || value != nil {
							return value, err
						}
					}
				} else {
					return metadataLookupBounded(merge, path, depth+1, visits)
				}
			}
		}
	}
	if node.Kind == yaml.SequenceNode {
		index, err := strconv.Atoi(path[0])
		if err == nil && index >= 0 && index < len(node.Content) {
			return metadataLookupBounded(node.Content[index], path[1:], depth+1, visits)
		}
	}
	return nil, nil
}

func metadataText(node *yaml.Node, depth int) (string, error) {
	visits := 0
	expandedBytes := 0
	return metadataTextBounded(node, depth, &visits, &expandedBytes)
}

func metadataTextBounded(node *yaml.Node, depth int, visits, expandedBytes *int) (string, error) {
	*visits++
	if *visits > 10000 {
		return "", fmt.Errorf("YAML 变量展开过大")
	}
	if depth > 64 {
		return "", fmt.Errorf("YAML 变量引用层级过深或存在循环")
	}
	if node.Kind == yaml.AliasNode {
		return metadataTextBounded(node.Alias, depth+1, visits, expandedBytes)
	}
	if node.Kind == yaml.ScalarNode {
		if node.Tag == "!!null" {
			return "", nil
		}
		*expandedBytes += len(node.Value)
		if *expandedBytes > 1<<20 {
			return "", fmt.Errorf("YAML 变量文字展开过大")
		}
		return node.Value, nil
	}
	if node.Kind == yaml.SequenceNode {
		parts := make([]string, len(node.Content))
		for i, child := range node.Content {
			value, err := metadataTextBounded(child, depth+1, visits, expandedBytes)
			if err != nil {
				return "", err
			}
			parts[i] = value
		}
		return strings.Join(parts, ", "), nil
	}
	return "", fmt.Errorf("YAML 变量是对象，请使用点分隔的字段名")
}
