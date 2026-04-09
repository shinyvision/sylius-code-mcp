<p align="center">
  <img src=".github/assets/sylius-code-mcp-logo.png" alt="sylius-code-mcp" width="200" />
</p>

<h1 align="center">Sylius Code MCP</h1>

<p align="center">
  An MCP server that gives AI coding assistants deep, context-aware tools for building on <a href="https://sylius.com">Sylius 2.x</a>.
</p>

---

Instead of asking an AI to guess at Sylius conventions and config file locations, sylius-code-mcp gives it precise, validated tools that read your actual project: compiled container, entity files, routing config, and translation files — before making any change. Every tool is idempotent and tells the LLM exactly what was done and what to do next.

## Tools

| Tool | What it does |
|---|---|
| `sylius_create_resource` | Scaffolds a complete Sylius resource: PHP entity, repository, Doctrine mapping, resource registration, and admin route in one shot |
| `sylius_create_resource_route` | Creates a `config/routes/_sylius/<resource>.yaml` route file and registers the import in `config/routes/sylius_admin.yaml` |
| `sylius_admin_create_grid` | Generates a `config/packages/sylius_grid/<resource>.yaml` grid, validates fields against the real entity, and auto-creates starter Twig templates for `twig`-type fields |
| `sylius_add_entity_fields` | Adds scalar fields and Doctrine associations to an existing PHP entity, generating PHP 8 ORM attributes, getters/setters, collection initialisation, and optionally the inverse side |
| `sylius_add_entity_index` | Adds a `#[ORM\Index]` to a PHP entity, deriving the index name automatically from the table and field names |
| `sylius_admin_add_menu_item` | Adds an item to the Sylius admin menu — reads the compiled container to find or create the correct event subscriber |
| `sylius_admin_get_items` | Returns the full current admin menu structure via PHP static analysis; use this before `sylius_admin_add_menu_item` to discover valid parent keys |
| `sylius_manage_translations` | Checks for missing translation keys, or adds/edits/removes entries in `translations/{domain}.{locale}.yaml`. I have found this one very, very useful! |
| `sylius_list_constraints` | Lists all Symfony validator constraints in the project, including constructor parameters; use before `sylius_add_entity_fields` to pick the right constraint names |

### Supported scalar types (entity fields)

`string` · `text` · `int` · `float` · `bool` · `datetime_immutable` · `date` · `json` · `decimal` · `uuid`

### Supported association types

`ManyToOne` · `OneToMany` · `ManyToMany` · `OneToOne`

### Supported grid field types

`string` · `datetime` · `boolean` · `money` · `image` · `twig`

## Requirements

- A Sylius 2.x project with a compiled Symfony container (`var/cache/<env>/App_KernelDevDebugContainer.xml`)
- Go 1.25+ (only needed if building from source)

## Installation

### Pre-built binaries

Download the latest binary for your platform from the [Releases](https://github.com/shinyvision/sylius-code-mcp/releases) page, then make it executable:

```sh
chmod +x sylius-code-mcp_*
```

### Build from source

```sh
git clone https://github.com/shinyvision/sylius-code-mcp
cd sylius-code-mcp
CGO_ENABLED=1 go build -o sylius-code-mcp .
```

## Configuration

The server requires the absolute path to your Sylius project root. Pass it either via the `-project` flag or the `SYLIUS_PROJECT_ROOT` environment variable.

---

### Claude Code

Add the server to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "sylius-code-mcp": {
      "command": "/absolute/path/to/sylius-code-mcp",
      "args": ["-project", "/absolute/path/to/your/sylius/project"]
    }
  }
}
```

---

### Codex

Add the server to `~/.codex/config.toml`:

```toml
[[mcp_servers]]
name    = "sylius"
command = "/absolute/path/to/sylius-code-mcp"
args    = ["-project", "/absolute/path/to/your/sylius/project"]
```

Or pass it inline when running Codex:

```sh
codex --mcp-server 'sylius:{"command":"/absolute/path/to/sylius-code-mcp","args":["-project","/absolute/path/to/your/sylius/project"]}' \
  "Add a ProductReview resource to the app"
```

---

### OpenCode

Add the server to your project-level `opencode.json` or `~/.config/opencode/config.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sylius": {
      "type": "local",
      "command": [
        "/absolute/path/to/sylius-code-mcp",
        "-project",
        "/absolute/path/to/your/sylius/project"
      ]
    }
  }
}
```

---
<div style="padding:10px; border-left:5px solid orange; background:#fff3cd; color: #1F1D2E">
  <strong>Note:</strong> Those Codex / OpenCode configurations may not work; I don’t use these programs myself. You may have to change it a bit. Please leave a PR if I need to change it.
</div>


## Usage example

Once configured, you can ask your coding assistant things like:

> "Create a `Supplier` resource under `App\Supplier`, add `name` (string) and `notes` (text, nullable) fields, add an index on `name`, and set up a grid with those two columns."

The assistant will call the tools in the correct order: `sylius_create_resource`, `sylius_add_entity_fields`, `sylius_add_entity_index`, `sylius_admin_create_grid`. Each time it will work from the actual state of your project files.

Maybe you have to add some instructions to your `AGENTS.md` or `CLAUDE.md` to let it know it should prefer using the MCP for doing Sylius specific things.

## License

MIT
