import { useEffect, useRef } from 'react';
import { basicSetup, EditorView } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';
import { oneDark } from '@codemirror/theme-one-dark';
import { javascript } from '@codemirror/lang-javascript';
import { python } from '@codemirror/lang-python';
import { java } from '@codemirror/lang-java';
import { cpp } from '@codemirror/lang-cpp';
import { go, goLanguage } from '@codemirror/lang-go';
import { completeFromList, ifNotIn } from '@codemirror/autocomplete';
import { LanguageSupport } from '@codemirror/language';
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
  javascript: () => javascript(),
  typescript: () => javascript({ typescript: true }),
  java: java,
  cpp: cpp,
};

function getLangExt(lang: string): Extension {
  return LANG_EXT[lang]?.() ?? [];
}

const editorTheme = EditorView.theme({
  '&': { height: '100%', fontSize: '14px' },
  '.cm-scroller': { overflow: 'auto' },
  '.cm-content': { paddingTop: '12px' },
  '.cm-focused': { outline: 'none' },
  '.cm-editor': { height: '100%' },
});

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
          themeCompartment.current.of(isDark ? oneDark : []),
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
      effects: themeCompartment.current.reconfigure(isDark ? oneDark : []),
    });
  }, [isDark]);

  return <div ref={containerRef} className="h-full" />;
}
