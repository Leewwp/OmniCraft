"use client";

interface ExamQuestion {
  id: number;
  question: {
    prompt: string;
    options: Record<string, string>;
  };
}

interface ExamQuestionProps {
  question: ExamQuestion;
  selectedKey: string;
  onSelect: (key: string) => void;
  disabled: boolean;
}

export default function ExamQuestion({ question, selectedKey, onSelect, disabled }: ExamQuestionProps) {
  const options = Object.entries(question.question.options);

  return (
    <div className="rounded-md border border-border bg-card p-6 shadow-none">
      <h3 className="text-sm font-semibold text-card-foreground">
        {question.question.prompt}
      </h3>
      <div className="mt-4 space-y-2">
        {options.map(([key, text]) => {
          const isSelected = selectedKey === key;
          return (
            <button
              key={key}
              type="button"
              disabled={disabled}
              onClick={() => onSelect(key)}
              className={`w-full rounded-md border px-4 py-3 text-left text-sm transition-all outline-none focus-visible:ring-2 focus-visible:ring-accent ${
                isSelected
                  ? "border-accent bg-accent/10 text-accent-foreground"
                  : "border-border bg-background text-foreground hover:bg-muted"
              } disabled:opacity-50 disabled:cursor-not-allowed`}
            >
              <span className="mr-2 font-medium text-muted-foreground">{key.toUpperCase()}.</span>
              {text}
            </button>
          );
        })}
      </div>
    </div>
  );
}
