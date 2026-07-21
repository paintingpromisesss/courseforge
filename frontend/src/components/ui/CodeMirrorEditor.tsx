import { useEffect, useRef } from 'react';
import { basicSetup, EditorView } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';
import { javascript } from '@codemirror/lang-javascript';
import { python } from '@codemirror/lang-python';
import { java } from '@codemirror/lang-java';
import { cpp } from '@codemirror/lang-cpp';
import { go, goLanguage } from '@codemirror/lang-go';
import { sql, PostgreSQL } from '@codemirror/lang-sql';
import { completeFromList, ifNotIn } from '@codemirror/autocomplete';
import { LanguageSupport, StreamLanguage, HighlightStyle, syntaxHighlighting, indentUnit } from '@codemirror/language';
import { tags as t } from '@lezer/highlight';
import { csharp } from '@codemirror/legacy-modes/mode/clike';
import type { Extension } from '@codemirror/state';

// @codemirror/lang-go omits built-in types and functions from keyword completions
const goBuiltins = completeFromList(
  [
    // types
    "bool", "byte", "complex64", "complex128", "error", "float32", "float64",
    "int", "int8", "int16", "int32", "int64", "rune", "string",
    "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
    // builtin functions
    "append", "cap", "clear", "close", "complex", "copy", "delete",
    "imag", "len", "make", "max", "min", "new", "panic", "print",
    "println", "real", "recover",
    // constants
    "true", "false", "iota", "nil",
  ].map(label => ({ label, type: "keyword" }))
);

const GO_DONT_COMPLETE = ["String", "Comment", "TemplateString"];

function goWithBuiltins(): LanguageSupport {
  const base = go();
  return new LanguageSupport(goLanguage, [
    base.support,
    goLanguage.data.of({ autocomplete: ifNotIn(GO_DONT_COMPLETE, goBuiltins) }),
  ]);
}

const LANG_EXT: Record<string, () => Extension> = {
  go: goWithBuiltins,
  python: python,
  python3: python,
  javascript: () => javascript(),
  typescript: () => javascript({ typescript: true }),
  java: java,
  cpp: cpp,
  csharp: () => StreamLanguage.define(csharp),
  postgres: () => sql({ dialect: PostgreSQL }),
};

// Indent unit per language: Go uses a real tab (gofmt rewrites files that way,
// and course templates already contain tab-indented Go source — inserting
// spaces there would visually misalign against the existing indentation).
// Others follow each language's common convention.
const INDENT_UNIT: Record<string, string> = {
  go: '\t',
  python: '    ',
  python3: '    ',
  java: '    ',
  cpp: '    ',
  csharp: '    ',
};

function getLangExt(lang: string): Extension {
  return [
    LANG_EXT[lang]?.() ?? [],
    indentUnit.of(INDENT_UNIT[lang] ?? '  '),
    EditorState.tabSize.of(4),
  ];
}

// Base chrome (background, gutters, selection, cursor, brackets) is themed
// entirely from the app's own CSS variables, so it's a single static
// extension that already matches both the dark and light app theme — no
// need to swap it when the user toggles theme, unlike the generic
// one-dark/no-theme split this replaced.
const BRAND = '#8251EE'; // keep in sync with tailwind.config.js colors.brand.DEFAULT
const editorTheme = EditorView.theme({
  '&': { height: '100%', fontSize: '13.5px', backgroundColor: 'var(--bg-1)' },
  '.cm-scroller': {
    overflow: 'auto',
    fontFamily: '"JetBrains Mono", ui-monospace, "Cascadia Code", Consolas, monospace',
    lineHeight: '1.6',
  },
  '.cm-content': { paddingTop: '14px', paddingBottom: '14px', caretColor: BRAND },
  '.cm-focused': { outline: 'none' },
  '.cm-editor': { height: '100%' },
  '.cm-gutters': { backgroundColor: 'var(--bg-1)', color: 'var(--tx-3)', border: 'none' },
  '.cm-lineNumbers .cm-gutterElement': { paddingRight: '14px' },
  '.cm-activeLineGutter': { backgroundColor: 'var(--bg-3)', color: 'var(--tx-2)' },
  '.cm-activeLine': { backgroundColor: 'var(--bg-2)' },
  '.cm-selectionBackground, .cm-content ::selection': { backgroundColor: 'rgba(130,81,238,0.25) !important' },
  '&.cm-focused .cm-selectionBackground': { backgroundColor: 'rgba(130,81,238,0.3) !important' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: BRAND, borderLeftWidth: '2px' },
  '.cm-matchingBracket, .cm-nonmatchingBracket': {
    backgroundColor: 'rgba(130,81,238,0.2)',
    outline: '1px solid rgba(130,81,238,0.4)',
  },
  '.cm-foldPlaceholder': { backgroundColor: 'var(--bg-4)', border: 'none', color: 'var(--tx-2)', borderRadius: '4px' },
  '.cm-tooltip': {
    backgroundColor: 'var(--bg-3)',
    border: '1px solid var(--bdr)',
    borderRadius: '6px',
  },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--bg-4)', color: 'var(--tx-1)' },
});

// Two syntax palettes (background/UI chrome above stays constant): dark
// keeps familiar One Dark-ish hues with a brand-tinted keyword color, light
// uses darker, higher-contrast tones readable on a white background.
const highlightDark = syntaxHighlighting(HighlightStyle.define([
  { tag: t.keyword, color: '#c58aff', fontWeight: '600' },
  { tag: [t.name, t.deleted, t.character, t.propertyName, t.macroName], color: 'var(--tx-1)' },
  { tag: [t.function(t.variableName), t.labelName], color: '#61afef' },
  { tag: [t.color, t.constant(t.name), t.standard(t.name)], color: '#d19a66' },
  { tag: [t.definition(t.name), t.separator], color: 'var(--tx-1)' },
  { tag: [t.typeName, t.className, t.number, t.changed, t.annotation, t.modifier, t.self, t.namespace], color: '#e5c07b' },
  { tag: [t.operator, t.operatorKeyword, t.url, t.escape, t.regexp, t.special(t.string)], color: '#56b6c2' },
  { tag: [t.meta, t.comment], color: 'var(--tx-3)', fontStyle: 'italic' },
  { tag: t.link, color: '#61afef', textDecoration: 'underline' },
  { tag: [t.atom, t.bool, t.special(t.variableName)], color: '#d19a66' },
  { tag: [t.processingInstruction, t.string, t.inserted], color: '#98c379' },
  { tag: t.invalid, color: '#f44747' },
]));

const highlightLight = syntaxHighlighting(HighlightStyle.define([
  { tag: t.keyword, color: BRAND, fontWeight: '600' },
  { tag: [t.name, t.deleted, t.character, t.propertyName, t.macroName], color: 'var(--tx-1)' },
  { tag: [t.function(t.variableName), t.labelName], color: '#0550ae' },
  { tag: [t.color, t.constant(t.name), t.standard(t.name)], color: '#b35900' },
  { tag: [t.definition(t.name), t.separator], color: 'var(--tx-1)' },
  { tag: [t.typeName, t.className, t.number, t.changed, t.annotation, t.modifier, t.self, t.namespace], color: '#953800' },
  { tag: [t.operator, t.operatorKeyword, t.url, t.escape, t.regexp, t.special(t.string)], color: '#0598a8' },
  { tag: [t.meta, t.comment], color: 'var(--tx-3)', fontStyle: 'italic' },
  { tag: t.link, color: '#0550ae', textDecoration: 'underline' },
  { tag: [t.atom, t.bool, t.special(t.variableName)], color: '#b35900' },
  { tag: [t.processingInstruction, t.string, t.inserted], color: '#0a7d3f' },
  { tag: t.invalid, color: '#cf222e' },
]));

interface Props {
  value: string;
  language: string;
  isDark: boolean;
  onChange: (value: string) => void;
}

export function CodeMirrorEditor({ value, language, isDark, onChange }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const langCompartment = useRef(new Compartment());
  const themeCompartment = useRef(new Compartment());
  const onChangeRef = useRef(onChange);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  // Create editor once on mount
  useEffect(() => {
    if (!containerRef.current) return;

    const view = new EditorView({
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          keymap.of([indentWithTab]),
          langCompartment.current.of(getLangExt(language)),
          themeCompartment.current.of(isDark ? highlightDark : highlightLight),
          editorTheme,
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
        ],
      }),
      parent: containerRef.current,
    });

    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Sync value from outside (e.g. reset, task change)
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== value) {
      view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
    }
  }, [value]);

  // Reconfigure language
  useEffect(() => {
    viewRef.current?.dispatch({
      effects: langCompartment.current.reconfigure(getLangExt(language)),
    });
  }, [language]);

  // Reconfigure theme
  useEffect(() => {
    viewRef.current?.dispatch({
      effects: themeCompartment.current.reconfigure(isDark ? highlightDark : highlightLight),
    });
  }, [isDark]);

  return <div ref={containerRef} className="h-full" />;
}
