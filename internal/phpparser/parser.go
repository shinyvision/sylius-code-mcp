package phpparser

import (
	"fmt"
	"strings"

	sitter "github.com/alexaandru/go-tree-sitter-bare"
)

type SubscribedEvent struct {
	Event  string
	Method string
}

func ParseSubscribedEvents(content string) []SubscribedEvent {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return nil
	}
	defer tree.Close()

	root := tree.RootNode()
	bodyNode := findMethodBody(root, "getSubscribedEvents", data)
	if bodyNode.IsNull() {
		return nil
	}

	var returnNode sitter.Node
	for i := uint32(0); i < bodyNode.NamedChildCount(); i++ {
		child := bodyNode.NamedChild(i)
		if child.Type() == "return_statement" {
			returnNode = child
			break
		}
	}
	if returnNode.IsNull() {
		return nil
	}

	var arrayNode sitter.Node
	for i := uint32(0); i < returnNode.NamedChildCount(); i++ {
		child := returnNode.NamedChild(i)
		if child.Type() == "array_creation_expression" {
			arrayNode = child
			break
		}
	}
	if arrayNode.IsNull() {
		return nil
	}

	return parseSubscribedEventsArray(arrayNode, data)
}

func findMethodBody(root sitter.Node, methodName string, data []byte) sitter.Node {
	node := findMethodNode(root, methodName, data)
	if node.IsNull() {
		return sitter.Node{}
	}
	return node.ChildByFieldName("body")
}

func parseSubscribedEventsArray(array sitter.Node, data []byte) []SubscribedEvent {
	var events []SubscribedEvent

	for i := uint32(0); i < array.NamedChildCount(); i++ {
		elem := array.NamedChild(i)
		if elem.Type() != "array_element_initializer" {
			continue
		}

		keyNode := arrayElemKey(elem)
		if keyNode.IsNull() {
			continue
		}
		eventName := extractStringValue(keyNode, data)
		if eventName == "" {
			continue
		}

		valNode := arrayElemValue(elem)
		if valNode.IsNull() {
			continue
		}
		method := extractMethodFromValue(valNode, data)
		if method == "" {
			continue
		}

		events = append(events, SubscribedEvent{Event: eventName, Method: method})
	}

	return events
}

func extractMethodFromValue(node sitter.Node, data []byte) string {
	if node.IsNull() {
		return ""
	}
	switch node.Type() {
	case "string", "encapsed_string":
		return extractStringValue(node, data)
	case "array_creation_expression":
		if node.NamedChildCount() == 0 {
			return ""
		}
		first := node.NamedChild(0)
		if first.Type() != "array_element_initializer" {
			return ""
		}
		val := arrayElemValue(first)
		return extractMethodFromValue(val, data)
	}
	return ""
}

func arrayElemKey(elem sitter.Node) sitter.Node {
	if elem.NamedChildCount() == 2 {
		return elem.NamedChild(0)
	}
	return sitter.Node{}
}

func arrayElemValue(elem sitter.Node) sitter.Node {
	count := elem.NamedChildCount()
	if count == 0 {
		return sitter.Node{}
	}
	if count == 1 {
		return elem.NamedChild(0)
	}
	return elem.NamedChild(1)
}

func extractStringValue(node sitter.Node, data []byte) string {
	if node.IsNull() {
		return ""
	}
	switch node.Type() {
	case "string":
		for i := uint32(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child.Type() == "string_content" {
				return child.Content(data)
			}
		}
		raw := strings.TrimSpace(node.Content(data))
		raw = strings.TrimPrefix(raw, "'")
		raw = strings.TrimSuffix(raw, "'")
		return raw
	case "encapsed_string":
		raw := strings.TrimSpace(node.Content(data))
		raw = strings.TrimPrefix(raw, "\"")
		raw = strings.TrimSuffix(raw, "\"")
		return raw
	}
	return ""
}

func ImplementsEventSubscriber(content string) bool {
	data := []byte(content)
	if !hasPhpTag(data) {
		data = append([]byte("<?php\n"), data...)
	}
	tree, err := parseTree(data)
	if err != nil {
		return false
	}
	defer tree.Close()

	return nodeContainsInterface(tree.RootNode(), data, "EventSubscriberInterface")
}

func hasPhpTag(data []byte) bool {
	return len(data) >= 2 && data[0] == '<' && data[1] == '?'
}

func nodeContainsInterface(root sitter.Node, data []byte, iface string) bool {
	stack := []sitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Type() == "class_interface_clause" {
			if strings.Contains(node.Content(data), iface) {
				return true
			}
			continue
		}

		for i := uint32(0); i < node.NamedChildCount(); i++ {
			stack = append(stack, node.NamedChild(i))
		}
	}
	return false
}

type MethodBounds struct {
	StartLine int
	EndLine   int
}

func FindMethodBounds(content, methodName string) (MethodBounds, bool) {
	data := []byte(content)
	tree, err := parseTree(data)
	if err != nil {
		return MethodBounds{}, false
	}
	defer tree.Close()

	node := findMethodNode(tree.RootNode(), methodName, data)
	if node.IsNull() {
		return MethodBounds{}, false
	}

	return MethodBounds{
		StartLine: int(node.StartPoint().Row),
		EndLine:   int(node.EndPoint().Row),
	}, true
}

func InsertBeforeMethodEnd(content, methodName, code string) (string, error) {
	bounds, ok := FindMethodBounds(content, methodName)
	if !ok {
		return "", fmt.Errorf("method %q not found", methodName)
	}

	lines := strings.Split(content, "\n")

	newLines := make([]string, 0, len(lines)+2)
	newLines = append(newLines, lines[:bounds.EndLine]...)
	if bounds.EndLine > 0 && strings.TrimSpace(lines[bounds.EndLine-1]) != "" {
		newLines = append(newLines, "")
	}
	newLines = append(newLines, code)
	newLines = append(newLines, lines[bounds.EndLine:]...)
	return strings.Join(newLines, "\n"), nil
}

func RenderMenuItemCode(parentKey, itemKey, label, route, icon string, weight int, routes []string) string {
	var sb strings.Builder

	if parentKey != "" {
		sb.WriteString(fmt.Sprintf("        $menu->getChild(%s)?->addChild(%s, ['route' => %s])\n",
			phpString(parentKey), phpString(itemKey), phpString(route)))
	} else {
		sb.WriteString(fmt.Sprintf("        $menu->addChild(%s, ['route' => %s])\n",
			phpString(itemKey), phpString(route)))
	}

	sb.WriteString(fmt.Sprintf("            ->setLabel(%s)\n", phpString(label)))
	if icon != "" {
		sb.WriteString(fmt.Sprintf("            ->setLabelAttribute('icon', %s)\n", phpString(icon)))
	}
	if weight != 0 {
		sb.WriteString(fmt.Sprintf("            ->setExtra('weight', %d)\n", weight))
	}
	if len(routes) > 0 {
		sb.WriteString("            ->setExtra('routes', [\n")
		for _, r := range routes {
			sb.WriteString(fmt.Sprintf("                ['route' => %s],\n", phpString(r)))
		}
		sb.WriteString("            ])\n")
	}

	result := strings.TrimRight(sb.String(), "\n")
	result += ";"
	return result
}

func phpString(s string) string {
	escaped := strings.ReplaceAll(s, `'`, `\'`)
	return "'" + escaped + "'"
}

func NewMenuSubscriberContent() string {
	return `<?php

declare(strict_types=1);

namespace App\EventListener\Menu;

use Sylius\Bundle\UiBundle\Menu\Event\MenuBuilderEvent;
use Symfony\Component\EventDispatcher\EventSubscriberInterface;

class MenuSubscriber implements EventSubscriberInterface
{
    public static function getSubscribedEvents(): array
    {
        return [
            'sylius.menu.admin.main' => ['addAdminMenuItems', -2048],
        ];
    }

    public function addAdminMenuItems(MenuBuilderEvent $event): void
    {
        $menu = $event->getMenu();
    }
}
`
}
