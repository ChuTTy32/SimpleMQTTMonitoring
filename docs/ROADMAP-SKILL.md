Analyze the current codebase. Do not modify product code. Only create or update files under docs/codemap/.

Ignore vendor, build, dist, cache, and other generated directories.

If docs/codemap/ already exists, first compare the existing codemap.lock with the current repo and list the modules that changed. Then regenerate these three files together:

1. docs/codemap/codemap.html

Build a fully self-contained interactive code map that opens directly in a browser. Include:

* Major modules, services, databases, queues, and external dependencies.  
* No more than 20 primary nodes; group low-level files under their parent module.  
* Calls and data flows between modules.  
* The 3-5 most important end-to-end flows.

Visual Style & UI Design:

* Strict, minimalist, premium, luxury aesthetic with a dark theme by default.  
* Palette: Monochromatic shades of dark gray, charcoal, and white, with 1-2 selective accent colors for emphasis (e.g., active elements, flow highlights).  
* Color-coded module types with a clean, simple legend adhering to the minimalist palette.  
* Collapsible navigation panel (sidebar or top bar) that can be hidden to maximize screen space.  
* Fullscreen roadmap mode (F shortcut or button) with fluid pan, zoom, and drag controls for smooth navigation on large diagrams.

Header & Metadata:

* At the top, show the repository name, generation timestamp, target commit hash, **project version**, **direct GitHub repository link**, and current **project status** (e.g., researching, development, preproduction, production).

About Panel:

* Add a collapsible "About the project" panel (accessible from the header, e.g. an info icon/button), separate from the architecture canvas.
* Content: project goal/task, team/authors, tech stack per module, current high-level status. Keep it short — a summary, not a full document.
* Source this content **only from `docs/PROJECT.md`** (and `docs/database-schema.md` if relevant) — never invent, guess, or infer these facts from code alone. `docs/PROJECT.md` is the single source of truth for this narrative info; the codemap must not fork a second, potentially conflicting copy of it.
* If a fact isn't present in `docs/PROJECT.md`, omit it rather than guessing — do not fabricate authors, goals, or stack details.
* End the panel with an explicit link/reference: "Full details: docs/PROJECT.md" (as a relative link, and as a GitHub link using the same `{org}/{repo}/blob/{commit}/` pattern as Direct Source Links) so readers know where the canonical version lives and that the panel is a summary, not a replacement.
* When regenerating the codemap, refresh this panel's content from the current `docs/PROJECT.md` — do not carry over stale text from a previous generation if the source file changed.

Layout & Interactive Features:

* System boundaries and the most important data flows must be visible on the first screen.  
* Automatic layout algorithm that minimizes crossing edges.  
* Level of Detail (LoD) toggle: switch between High-Level (core services, DBs, queues) and Low-Level (expanded sub-modules and entrypoints).  
* Interactive selection: clicking any module highlights its upstream callers, downstream dependencies, related tests, and the flows it belongs to.  
* Selection of a flow highlights its complete end-to-end path.  
* Blast Radius / Impact Analysis: when a module is selected, visually dim/fade unrelated elements.  
* Architectural Drift / Delta View: visually highlight nodes and connections that have changed or drifted compared to the previous codemap.lock.  
* Deep Linking (URL Hash): sync state with the URL (e.g., \#node=id or \#flow=id) to allow direct sharing of specific architectural views.  
* Minimap in the corner for easy orientation when zoomed in.  
* Keyboard Shortcuts: / to focus search, Esc to clear selection, F for fullscreen, 1-5 to trigger main E2E flows, R to reset zoom/fit to view.  
* Direct Source Links: in the side panel details, provide clickable URLs pointing directly to source files and line numbers on GitHub (https://github.com/{org}/{repo}/blob/{commit}/{path}\#L{line}).  
* Diagram Export: action to export the current view state to SVG, PNG, or Mermaid.js markup.  
* Search, filtering, zoom, and drag controls built-in.  
2. docs/codemap/codemap.json

Use this structure:

{

"generated\_at": "",

"generated\_from\_commit": "",

"scope": \[\],

"nodes": \[\],

"edges": \[\],

"flows": \[\]

}

Each node must include:

* id  
* path  
* role  
* entrypoints  
* tests  
* constraints  
* evidence

Each edge must include:

* from  
* to  
* type  
* evidence

type may only be:

* imports  
* calls  
* reads  
* writes  
* publishes  
* subscribes

Each flow must include:

* trigger  
* steps  
* outcome

Every step must reference an existing node id.

Attach the matching source path and symbol to every node and edge. Mark any relationship without source evidence as unknown. Do not guess.

3. docs/codemap/codemap.lock

Use parseable JSON to record:

* the current commit  
* whether the working tree has uncommitted changes  
* generation time  
* scanned scope  
* excluded directories  
* the fingerprint algorithm  
* a deterministic fingerprint for each top-level module, calculated from its tracked file paths and current file contents

If no existing codemap.lock is found, treat every module as new and generate the full map.

When finished, verify that:

* codemap.json parses successfully  
* every node path exists and every evidence symbol can be found in the source  
* every edge and flow step references an existing node  
* codemap.html and codemap.json use the same nodes, edges, and flows  
* codemap.lock matches the current commit, working tree state, and module fingerprints  
* every relationship without source evidence is marked unknown

These three files must always be generated together from the current repo. Never edit only one of them manually.

Finally, show:

* files created or modified  
* stale modules  
* remaining unknowns  
* validation results  
* the complete diff