package syliusmenu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylius-code-mcp/internal/container"
	"github.com/sylius-code-mcp/internal/phpparser"
)

const menuEvent = "sylius.menu.admin.main"
const defaultSubscriberPath = "src/EventListener/Menu/MenuSubscriber.php"

type Params struct {
	ParentKey string
	ItemKey   string
	Label     string
	Route     string
	Icon      string
	Weight    int
}

type Result struct {
	FilePath    string
	FileCreated bool
	Message     string
}

type MenuHandlerInfo struct {
	ServiceID  string
	Class      string
	FilePath   string
	MethodName string
}

func FindMenuHandlers(projectRoot string, cnt *container.Container, psr4 container.PSR4Map) []MenuHandlerInfo {
	var handlers []MenuHandlerInfo
	seen := make(map[string]bool)

	addHandler := func(h MenuHandlerInfo) {
		if !seen[h.FilePath] {
			seen[h.FilePath] = true
			handlers = append(handlers, h)
		}
	}

	if cnt == nil {
		return handlers
	}

	for _, svc := range cnt.ServicesWithTag("kernel.event_listener") {
		if svc.TagAttr("kernel.event_listener", "event") != menuEvent {
			continue
		}
		method := svc.TagAttr("kernel.event_listener", "method")
		if method == "" {
			method = "__invoke"
		}
		filePath, ok := container.ResolveClass(svc.Class, psr4, projectRoot)
		if !ok {
			continue
		}
		addHandler(MenuHandlerInfo{
			ServiceID:  svc.ID,
			Class:      svc.Class,
			FilePath:   filePath,
			MethodName: method,
		})
	}

	for _, svc := range cnt.ServicesWithTag("kernel.event_subscriber") {
		filePath, ok := container.ResolveClass(svc.Class, psr4, projectRoot)
		if !ok {
			continue
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := string(data)
		for _, ev := range phpparser.ParseSubscribedEvents(content) {
			if ev.Event == menuEvent {
				addHandler(MenuHandlerInfo{
					ServiceID:  svc.ID,
					Class:      svc.Class,
					FilePath:   filePath,
					MethodName: ev.Method,
				})
				break
			}
		}
	}

	return handlers
}

func AddMenuItem(projectRoot string, p Params) (Result, error) {
	if err := validateParams(p); err != nil {
		return Result{}, err
	}

	psr4, err := container.LoadPSR4(projectRoot)
	if err != nil {
		return Result{}, fmt.Errorf("loading PSR-4 autoload map: %w", err)
	}

	cnt, containerErr := container.Load(projectRoot)
	if containerErr != nil {
		cnt = nil
	}

	handlers := FindMenuHandlers(projectRoot, cnt, psr4)

	var filePath string
	var methodName string
	created := false

	if len(handlers) == 0 {
		filePath = filepath.Join(projectRoot, defaultSubscriberPath)
		methodName = "addAdminMenuItems"

		if err := ensureMenuSubscriber(filePath); err != nil {
			return Result{}, fmt.Errorf("creating MenuSubscriber: %w", err)
		}
		created = true
	} else {
		h := pickPreferredHandler(handlers)
		filePath = h.FilePath
		methodName = h.MethodName
	}

	routes := DiscoverResourceRoutes(projectRoot, p.Route)

	code := phpparser.RenderMenuItemCode(p.ParentKey, p.ItemKey, p.Label, p.Route, p.Icon, p.Weight, routes)
	if err := insertMenuItemInFile(filePath, methodName, code); err != nil {
		return Result{}, err
	}

	msg := buildResultMessage(projectRoot, filePath, p, created)
	return Result{
		FilePath:    filePath,
		FileCreated: created,
		Message:     msg,
	}, nil
}

func ensureMenuSubscriber(filePath string) error {
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(phpparser.NewMenuSubscriberContent()), 0644)
}

func insertMenuItemInFile(filePath, methodName, code string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	updated, err := phpparser.InsertBeforeMethodEnd(string(data), methodName, code)
	if err != nil {
		return fmt.Errorf("inserting menu item into %s (method %q): %w", filePath, methodName, err)
	}

	return os.WriteFile(filePath, []byte(updated), 0644)
}

func pickPreferredHandler(handlers []MenuHandlerInfo) MenuHandlerInfo {
	for _, h := range handlers {
		if strings.HasPrefix(h.Class, `App\`) {
			return h
		}
	}
	return handlers[0]
}

func validateParams(p Params) error {
	if strings.TrimSpace(p.ItemKey) == "" {
		return fmt.Errorf("itemKey is required")
	}
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("labelTranslationKey is required")
	}
	if strings.TrimSpace(p.Route) == "" {
		return fmt.Errorf("route is required")
	}
	return nil
}

func buildResultMessage(projectRoot, filePath string, p Params, created bool) string {
	rel, err := filepath.Rel(projectRoot, filePath)
	if err != nil {
		rel = filePath
	}

	var action string
	if created {
		action = fmt.Sprintf("Created new MenuSubscriber at %s and added", rel)
	} else {
		action = fmt.Sprintf("Added")
	}

	var location string
	if p.ParentKey != "" {
		location = fmt.Sprintf("under parent menu item %q", p.ParentKey)
	} else {
		location = "to the root admin menu"
	}

	return fmt.Sprintf(
		"%s menu item %q (%s) with route %q %s in %s.\n\n"+
			"Next steps:\n"+
			"1. Clear the Symfony cache: php bin/console cache:clear\n"+
			"2. Verify the menu item appears in the admin panel.\n"+
			"3. Adjust weight/ordering in %s as needed.\n"+
			"4. The file is now yours to maintain and modify directly as required.",
		action, p.ItemKey, p.Label, p.Route, location, rel, rel,
	)
}
