from __future__ import annotations

import importlib

from prompt_toolkit.key_binding import KeyBindings
from prompt_toolkit.key_binding import merge_key_bindings


def build_global_cancel_key_bindings() -> KeyBindings:
    """
    Global key bindings for Questionary (prompt_toolkit) prompts.

    - Ctrl+C: abort immediately (KeyboardInterrupt)
    - Esc: abort immediately (KeyboardInterrupt)
    """
    bindings = KeyBindings()

    @bindings.add("c-c", eager=True)
    @bindings.add("escape", eager=True)
    def _(event) -> None:
        event.app.exit(exception=KeyboardInterrupt)

    return bindings


def _wrap_prompt_toolkit_callable(factory, global_bindings: KeyBindings):
    if getattr(factory, "_peg_this_cancel_wrapped", False):
        return factory

    def _wrapped(*args, **kwargs):
        existing = kwargs.get("key_bindings")
        if existing is None:
            kwargs["key_bindings"] = global_bindings
        else:
            kwargs["key_bindings"] = merge_key_bindings([existing, global_bindings])
        return factory(*args, **kwargs)

    _wrapped._peg_this_cancel_wrapped = True  # type: ignore[attr-defined]
    _wrapped._peg_this_cancel_original = factory  # type: ignore[attr-defined]
    return _wrapped


def _patch_module_attr(module_name: str, attr_name: str, global_bindings: KeyBindings) -> None:
    try:
        module = importlib.import_module(module_name)
    except Exception:
        return

    if not hasattr(module, attr_name):
        return

    original = getattr(module, attr_name)
    setattr(module, attr_name, _wrap_prompt_toolkit_callable(original, global_bindings))


def install_global_questionary_cancel_handling() -> None:
    """
    Ensure Ctrl+C / Esc always abort Questionary prompts, consistently.

    - Merge global cancel bindings into Questionary prompt-toolkit apps/sessions.
    - Make `.ask()` behave like `.unsafe_ask()` so KeyboardInterrupt isn't swallowed.
    """
    import questionary
    from questionary.question import Question

    if getattr(install_global_questionary_cancel_handling, "_installed", False):
        return

    global_bindings = build_global_cancel_key_bindings()

    if not getattr(Question.ask, "_peg_this_patched", False):

        def ask(self, *args, **kwargs):
            return self.unsafe_ask(*args, **kwargs)

        ask._peg_this_patched = True  # type: ignore[attr-defined]
        Question.ask = ask  # type: ignore[assignment]

    # Questionary prompt implementations often pass `key_bindings=` explicitly, and also
    # forward **kwargs to prompt-toolkit's Application/PromptSession. Injecting our own
    # `key_bindings` via Questionary kwargs can therefore cause "multiple values"
    # errors. Instead, wrap the local Application/PromptSession call sites and merge.
    _patch_module_attr("questionary.prompts.select", "Application", global_bindings)
    _patch_module_attr("questionary.prompts.checkbox", "Application", global_bindings)
    _patch_module_attr("questionary.prompts.rawselect", "Application", global_bindings)

    _patch_module_attr("questionary.prompts.text", "PromptSession", global_bindings)
    _patch_module_attr("questionary.prompts.confirm", "PromptSession", global_bindings)
    _patch_module_attr("questionary.prompts.press_any_key_to_continue", "PromptSession", global_bindings)
    _patch_module_attr("questionary.prompts.path", "PromptSession", global_bindings)
    _patch_module_attr("questionary.prompts.autocomplete", "PromptSession", global_bindings)
    _patch_module_attr("questionary.prompts.common", "PromptSession", global_bindings)

    install_global_questionary_cancel_handling._installed = True  # type: ignore[attr-defined]
