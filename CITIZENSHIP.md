# Citizenship preparation over MCP

Citizenship mode turns the assets in `bgtutor-assets/citizenship-test/` into a
live class led by the connected AI. The server supplies the curriculum,
lessons, questions, exact answer-key grading, vocabulary and saved progress.
The AI listens, explains, asks follow-up questions and adapts the lesson. The
server makes no LLM API calls.

```bash
go build -o bgtutor ./cmd/bgtutor
./bgtutor validate --mode citizenship --data-dir ../bgtutor-assets
./bgtutor serve --mode citizenship --data-dir ../bgtutor-assets
# Or set BGTUTOR_MODE=citizenship and BGTUTOR_DATA_DIR=../bgtutor-assets.
```

Connect the voice AI to `/mcp`, then ask it to start citizenship preparation.
The mode is chosen when starting the server; podcast remains the default.
Citizenship mode exposes eight study/exam tools and the three vocabulary
tools. Authentication and Streamable HTTP work as in podcast mode.

## The teaching plan

1. **Diagnose.** Offer one complete sample paper, collect the answer sheet and
   grade it. Keep the remaining official papers for later fresh mocks. Use
   `latest_exams[].weak_topics` to pick the next drill.
2. **Teach the rules.** Work through every relevant study section, especially
   the individual chapters in `study/grammar-rules/`. Cover grammar, synonyms,
   antonyms, idioms, spelling and punctuation. Prioritise weaknesses without
   skipping the rest of the curriculum.
3. **Practise reading.** Use each test's passage and any table, and the narrated videos' subtitle segments. Ask questions
   that check who, what, when, quantities, comparisons and abbreviations.
   Keep official wording unchanged when presenting an official question.
4. **Prepare the application.** Work through the guide, official materials and
   candidate experiences: process, documents, practical preparation and known
   pitfalls. Distinguish official sources from candidate reports and retain
   the guide's dates and uncertainty. The sources list, chapter index and video-production README are
   available references; they are excluded from mastery totals.
5. **Rehearse the interview.** Use `study/interview` and the guide to practise
   spontaneous personal answers in Bulgarian about motivation, residence,
   work, family and daily life. This is separate from the written exam.
6. **Sit mocks.** Use complete practice and official sample papers, a learner's
   external 60-minute timer and a 20-answer sheet. Give no hints or feedback
   until submission. Grade against the supplied key and then teach the errors.
7. **Review and resume.** Save difficult words and rules, mark sections for
   review or mastered after testing, and revisit mistakes later. Aim for
   strong independent recall and consistent results on fresh papers.

The server sends this workflow on MCP initialization. The AI should start by
calling `get_citizenship_plan` and `list_citizenship_materials`, so it can resume
existing progress and use the currently available assets.

## What a live class feels like

Keep a class around 15–25 minutes unless the learner prefers otherwise:

- Ask a warm-up question and listen to the response before explaining.
- Introduce one useful example and invite the learner to notice the pattern.
- Explain briefly, using English when useful, then return to Bulgarian.
- Ask **one question at a time** and wait. Respond to the actual answer.
- If the learner struggles, give the smallest useful hint and let them repair
  the sentence. Model an answer after an attempt, then try a fresh example.
- Gradually remove support. Ask for personal examples and an explanation of
  the rule. Show or spell distinctions that cannot be heard in voice mode.
- Recheck a mistake after another activity and in a later class. Reading a
  chapter or repeating the tutor's answer alone does not demonstrate mastery.

Each grammar chapter contains its goal, rule, examples, opening activity,
guided exercises, private tutor answer notes, hints and independent-use checks.
The AI follows those instructions instead of reading the chapter aloud. A
chapter can take several classes; the progress note records what remains.

Open-ended grammar and interview answers are evaluated by the AI against the
retrieved material. The server supplies objective grading only for the
prepared multiple-choice tests. Generated exercises are clearly tutor practice.

## Tool sequence

| Tool | Input | Result / behavior |
|---|---|---|
| `list_citizenship_materials` | none | Documents with section counts; tests with source, question count, readiness and validation errors |
| `get_citizenship_plan` | none | Phases, every relevant section's `unseen`/`review`/`mastered` status, mastery totals and latest mock scores/weak topics |
| `get_citizenship_lesson` | `material_id`, `index` | One Markdown section or timestamped video caption, position, `is_last`, `next_index`, optional relative `media_path`; refuses test files |
| `record_citizenship_progress` | `material_id`, `index`, `status`, optional `note` | Saves tutor-assessed `review` or `mastered` after testing recall |
| `get_citizenship_question` | `test_id`, `index` | One question, topic, four options and the reading passage for questions 1–5; no key |
| `check_citizenship_answer` | `test_id`, `index`, `answer` | Immediate practice feedback, correct letter and any prepared explanation |
| `get_citizenship_exam` | `test_id` | Complete 20-question paper without keys; 60-minute timer guidance and a 12/20 practice pass mark |
| `grade_citizenship_exam` | `test_id`, `answers` | Grades a complete sheet, returns ordered feedback and saves the latest score and weak topics |

The standard `save_vocabulary`, `list_vocabulary` and `delete_vocabulary` tools
are also available. Use optional episode fields only for podcast sources; for
citizenship terms put the relevant chapter/rule in `note`.

IDs are relative paths **without file extensions**, for example
`study/grammar-rules/05-masculine-article`, `practice/test-01` and
`official-samples/variant_1`. Get IDs from the catalog. Section and question
indexes start at 1.

```json
{
  "test_id": "official-samples/variant_1",
  "answers": [
    {"index": 1, "answer": "Г"},
    {"index": 2, "answer": "В"}
  ]
}
```

This abbreviated example is not a valid full submission: grading requires
exactly 20 distinct indexes. Use an empty `answer` for an unanswered question.
Letters must be Cyrillic `А`, `Б`, `В`, `Г` (lowercase and surrounding whitespace
are accepted). The AI maps a spoken option to its unambiguous Bulgarian letter.

The API is stateless about active classes and exams. Withholding practice
feedback during a mock is a tutor instruction, not an access restriction: the
same learner can ask to leave a mock and switch to guided practice. Timers are
external; the server does not claim to measure exam time.

## Content formats

Only prepared assets live in the assets repo. The server repo's tests use
synthetic fixtures and do not include the real exam questions.

- Study documents are Markdown files under `citizenship-test/`. `##` headings
  start sections; `###` headings stay in the same section. The text before the
  first `##` is also a section when nonempty. A rule chapter uses one `#` title
  and `###` subheadings, keeping the whole live lesson together.
- Narrated videos have `.srt` captions. Each sequential subtitle cue becomes
  a listening section with its timestamp and a relative `.mp4` media path.
  The MCP server returns the transcript and reference; it does not stream
  video. The tutor can read the caption naturally if the learner is not
  watching the local video.
- Original practice papers are `practice/test-*.md` in the incoming assets
  format: bold numbered questions (`**1. …**`), four Cyrillic options, followed
  by `## Отговори и обяснения`, a 20-column key row and numbered explanations.
  Section headings are retained in prompts for otherwise bare questions.
- Official PDF originals remain in `official-samples/`. The server reads their
  prepared JSON counterparts, not PDF files; no PDF tools are needed at runtime.
  Every JSON file under `citizenship-test/` is treated as a prepared test.

A prepared JSON test uses this schema (the abbreviated arrays must contain
20 records in a real file):

```json
{
  "format_version": 1,
  "title": "Official sample — variant 1 (2013)",
  "source": "Official source URL and original PDF path",
  "reading": "Original reading passage, including its table",
  "questions": [
    {
      "index": 1,
      "topic": "reading",
      "prompt": "Question in Bulgarian",
      "choices": [
        {"letter": "А", "text": "First option"},
        {"letter": "Б", "text": "Second option"},
        {"letter": "В", "text": "Third option"},
        {"letter": "Г", "text": "Fourth option"}
      ]
    }
  ],
  "answers": [
    {"index": 1, "answer": "Б", "explanation": "Optional prepared explanation"}
  ]
}
```

Validation requires a version, title, source, reading passage, exactly 20
sequential questions/keys, topics, prompts and four nonempty options in
А/Б/В/Г order. Invalid tests remain visible in the catalog with a reason and
cannot be served or graded. Official keys need not contain explanations;
the AI can explain the marked result afterward.

## Saved learner data

`--data-dir` holds the following sibling folders:

```text
episodes/                  optional in citizenship mode
citizenship-test/          prepared content from bgtutor-assets
vocabulary/saved.json      shared learner notebook
citizenship-progress/saved.json
```

Progress format version 1 stores section assessments, notes, content hashes,
timestamps and the latest score/weak topics per test. If a section changes,
its old assessment no longer counts as mastered. Corrupt/unsupported progress
files return an error instead of being silently overwritten. Writes are
atomic and serialized within the server process. This is a single learner's
server, with one process writing the personal files.

Keep the personal folders ignored by Git and writable in the mounted data
volume. Lesson content may be read-only. The server supports a symlinked
`citizenship-test` root. Files are reloaded on each call, so newly prepared
content appears without a restart.
