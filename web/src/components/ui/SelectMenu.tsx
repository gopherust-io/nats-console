import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import "../../styles/select-menu.css";

export type SelectMenuOption = {
  value: string;
  label: string;
  description?: string;
};

type SelectMenuProps = {
  id?: string;
  value: string;
  options: SelectMenuOption[];
  placeholder?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  className?: string;
  "aria-label"?: string;
  size?: "md" | "sm";
};

function initialOf(label: string) {
  const ch = label.trim().charAt(0);
  return ch ? ch.toUpperCase() : "?";
}

export default function SelectMenu({
  id,
  value,
  options,
  placeholder = "Select…",
  disabled = false,
  onChange,
  className = "",
  "aria-label": ariaLabel,
  size = "md",
}: SelectMenuProps) {
  const autoId = useId();
  const listId = `${autoId}-list`;
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);

  const selected = options.find((o) => o.value === value);
  const display = selected?.label ?? placeholder;
  const isPlaceholder = !selected;
  const showMarks = options.some((o) => o.description);

  useEffect(() => {
    if (!open) return;
    const onDoc = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const idx = options.findIndex((o) => o.value === value);
    setActiveIndex(idx >= 0 ? idx : 0);
  }, [open, options, value]);

  function pick(next: string) {
    onChange(next);
    setOpen(false);
  }

  function onTriggerKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return;
    if (event.key === "ArrowDown" || event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      setOpen(true);
    }
  }

  function onListKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((i) => Math.min(options.length - 1, Math.max(0, i) + 1));
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((i) => Math.max(0, (i < 0 ? 0 : i) - 1));
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      const opt = options[activeIndex];
      if (opt) pick(opt.value);
      return;
    }
    if (event.key === "Home") {
      event.preventDefault();
      setActiveIndex(0);
      return;
    }
    if (event.key === "End") {
      event.preventDefault();
      setActiveIndex(options.length - 1);
    }
  }

  return (
    <div
      ref={rootRef}
      className={`nc-select${size === "sm" ? " nc-select--sm" : ""}${open ? " nc-select--open" : ""}${showMarks ? " nc-select--rich" : ""}${className ? ` ${className}` : ""}`}
    >
      <button
        type="button"
        id={id}
        className={`nc-select__trigger${isPlaceholder ? " nc-select__trigger--placeholder" : ""}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-label={ariaLabel}
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
        onKeyDown={onTriggerKeyDown}
      >
        {selected && showMarks ? (
          <span className="nc-select__mark" aria-hidden="true">
            {initialOf(selected.label)}
          </span>
        ) : null}
        <span className="nc-select__value">
          <span className="nc-select__label">{display}</span>
          {selected?.description ? (
            <span className="nc-select__desc">{selected.description}</span>
          ) : null}
        </span>
        <span className="nc-select__chevron" aria-hidden="true">
          <svg width="10" height="10" viewBox="0 0 12 12" fill="none">
            <path
              d="M2.5 4.25 6 7.75 9.5 4.25"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </span>
      </button>

      {open && (
        <div
          id={listId}
          className="nc-select__menu"
          role="listbox"
          tabIndex={-1}
          aria-activedescendant={
            activeIndex >= 0 ? `${listId}-opt-${activeIndex}` : undefined
          }
          onKeyDown={onListKeyDown}
        >
          {options.length === 0 ? (
            <div className="nc-select__empty">{placeholder}</div>
          ) : (
            options.map((opt, index) => {
              const isSelected = opt.value === value;
              const isActive = index === activeIndex;
              return (
                <button
                  key={opt.value}
                  type="button"
                  id={`${listId}-opt-${index}`}
                  role="option"
                  aria-selected={isSelected}
                  className={`nc-select__option${isSelected ? " nc-select__option--selected" : ""}${isActive ? " nc-select__option--active" : ""}`}
                  onMouseEnter={() => setActiveIndex(index)}
                  onClick={() => pick(opt.value)}
                >
                  {showMarks ? (
                    <span className="nc-select__mark" aria-hidden="true">
                      {initialOf(opt.label)}
                    </span>
                  ) : null}
                  <span className="nc-select__option-text">
                    <span className="nc-select__option-label">{opt.label}</span>
                    {opt.description ? (
                      <span className="nc-select__option-desc">{opt.description}</span>
                    ) : null}
                  </span>
                  <span className="nc-select__check" aria-hidden="true">
                    {isSelected ? (
                      <svg width="12" height="12" viewBox="0 0 14 14" fill="none">
                        <path
                          d="M2.5 7.2 5.6 10.2 11.5 3.8"
                          stroke="currentColor"
                          strokeWidth="1.75"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    ) : null}
                  </span>
                </button>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
