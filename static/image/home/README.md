# Homepage design assets

These original PNG assets were exported from the MasterGo **消息服务首页** layer
(56:8765, file 199699779279775) using its browser export controls.

Design reference: https://mastergo.com/goto/WaKvOEXx?page_id=M&layer_id=56:8765&file=199699779279775

- hero-background.png: the 1920 × 487 background group, exported at 2×. Its alpha channel supplies the fade into the page background.
- logo.png: the complete 201 × 50 brand lockup, exported at 2×.
- tech-background.png: the masked 1920 × 200 technology band, exported at 1×.
- The nine feature icons: original 56 × 56 image layers, exported at 2×.

Assets are served at /image/home/ through the existing static-file configuration
and embedded in the Go binary by main.go. The HTML remains semantic, selectable,
and responsive; complete-page exports are only visual references, not page content.
