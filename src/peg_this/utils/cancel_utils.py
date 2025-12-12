from __future__ import annotations

from prompt_toolkit.key_binding import KeyBindings


def build_global_cancel_key_bindings() -> KeyBindings:
    """
    Global key bindings for Questionary (prompt_toolkit) prompts.

    - Ctrl+C: abort immediately (KeyboardInterrupt)
    - Esc: abort immediately (KeyboardInterrupt)
    """
    bindings = KeyBindings()

    @bindings.add("c-c")
    @bindings.add("escape")
    def _(event) -> None:
        event.app.exit(exception=KeyboardInterrupt())

    return bindings


def install_global_questionary_cancel_handling() -> None:
    """
    Ensure Ctrl+C / Esc always abort Questionary prompts, consistently.

    - Inject global `key_bindings` into commonly-used Questionary prompt helpers.
    - Make `.ask()` behave like `.unsafe_ask()` so KeyboardInterrupt isn't swallowed.
    """
    import questionary
    from questionary.question import Question

    if getattr(install_global_questionary_cancel_handling, "_installed", False):
        return

    global_bindings = build_global_cancel_key_bindings()

    def _wrap_factory(factory):
        def _wrapped(*args, **kwargs):
            kwargs.setdefault("key_bindings", global_bindings)
            return factory(*args, **kwargs)

        return _wrapped

    for name in ("select", "text", "confirm", "checkbox", "press_any_key_to_continue"):
        if hasattr(questionary, name):
            setattr(questionary, name, _wrap_factory(getattr(questionary, name)))

    if not getattr(Question.ask, "_peg_this_patched", False):

        def ask(self, *args, **kwargs):
            return self.unsafe_ask(*args, **kwargs)

        ask._peg_this_patched = True  # type: ignore[attr-defined]
        Question.ask = ask  # type: ignore[assignment]

    install_global_questionary_cancel_handling._installed = True  # type: ignore[attr-defined]

