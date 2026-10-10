---
title: "Hotkeys"
description: "Keys and joystick buttons the player presses in the simulator, bound to an add-on's actions with pkg/hotkeys over SimConnect input groups: bind by name, rebind from a settings file, mapped again after a reconnect."
order: 16
section: "packages"
---

# Hotkeys

`pkg/hotkeys` binds keys and joystick buttons the player presses in the simulator to an add-on's actions, for example "answer the current checklist item" on Ctrl+Shift+C or a yoke button. It is built on SimConnect's input groups: `MapInputEventToClientEvent`, `SetInputGroupPriority`, `SetInputGroupState`, `RemoveInputEvent` and `ClearInputGroup`, which the engine and the manager also expose directly.

```go
h := hotkeys.New(0) // IDs from hotkeys.DefaultIDBase (0xB100)
h.Bind("checklistItem", "Ctrl+Shift+C", func() { crew.AnswerItem() })
h.SetBindings(loadedFromSettings) // the player's own bindings, by action name
h.Attach(client)                  // on every connect and reconnect
for msg := range client.Stream() {
    if h.Handle(msg) {
        continue
    }
    // …
}
save(h.Bindings()) // map[action]definition for the settings file
```

- A definition is SimConnect's input string: keys joined with `+` (`"Ctrl+Shift+C"`, `"VK_F12"`), or a joystick input (`"joystick:0:button:5"`). Which keys MSFS 2024 passes to SimConnect, and its joystick numbering, still need a live check.
- `Bind` before or after `Attach`. Binding a name again moves the action to the new input: the old one is removed and the new one mapped.
- `Attach` maps every binding on the connection, with the group at the highest priority and on. Call it again after a reconnect.
- `Enable(false)` turns the group off, so the keys reach the simulator as usual.
- `Handle` calls the action of a pressed input (outside the lock) and reports whether the message was one of its own.

IDs: the input group is the ID base and the actions' client events are the base + 1 onwards, up to `MaxActions` (64). The default base, 0xB100, is clear of the traffic World's 0xA000 range. Give another base to `New` to move it.
