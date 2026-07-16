# China Secondary-School Dataset Inventory By Subject

Status: research inventory only; product readiness remains `NOT_READY`.

Audit date: 2026-07-15.

## Decision First

No public source found in this audit is currently approved for commercial grading-model training. The strongest authentic scoring datasets are noncommercial, competition-only, unlicensed, or have unresolved provenance. Question banks can test subject knowledge and deterministic graders after legal review, but they do not teach a model how Chinese teachers award partial credit to real student answers.

The product must keep four evidence classes separate:

| Evidence class | Meaning | Current use |
| --- | --- | --- |
| Product-approved grading data | Commercial rights, anonymized real responses, teacher scores, Rubric and audit trail | None available |
| Restricted real grading data | Authentic student responses and labels, but noncommercial or competition-only | Research or licensing outreach only |
| Question/reference-answer data | Questions, correct answers and sometimes worked solutions | Knowledge checks, rule graders and Rubric drafting only |
| Synthetic/generated data | Generated wrong answers, explanations or scoring outputs | Development and adversarial coverage only; never Pilot Gold |

Repository availability and a repository-level license do not prove that exam papers, textbooks, teaching aids, student essays or republished model answers were validly relicensed.

## Subject Summary

| Subject | Best authentic candidate | Has real student response | Has teacher score | Commercial training now | Main missing evidence |
| --- | --- | ---: | ---: | --- | --- |
| Chinese | CCL CEFE/CERRE, AES-Dataset, CSEE-style essay resources | Yes | Some sources | No | Reading short answers, stable Rubrics, double scoring, consent and rights |
| Mathematics | M3KE, GAOKAO-Bench, Gaokao-LLM-data | No | No | No | Real steps, misconception labels and teacher-awarded partial credit |
| English | CSEE | Yes | Yes, four score fields | No, CC BY-NC 4.0 | Commercial license, more prompts, junior-middle writing and double scoring |
| Physics | M3KE, GAOKAO-Bench, Gaokao-LLM-data | No | No | No | Real calculations, units, diagrams, experiment answers and step scores |
| Chemistry | C-MHChem, GAOKAO-Bench, chemistry instruction data | No | No | No | Real equations, experiments, error corrections and teacher step scores |
| Biology | M3KE, GAOKAO-Bench | No | No | No | Real terminology/process answers and evidence-linked teacher scores |
| Politics | M3KE, GAOKAO-Bench | No | No | No | Material-analysis answers, current-policy versioning and teacher Rubrics |
| History | M3KE, GAOKAO-Bench | No | No | No | Source-based arguments, chronology/causality evidence and partial credit |
| Geography | M3KE, GAOKAO-Bench | No | No | No | Map/chart answers, spatial reasoning evidence and teacher partial credit |

`Politics` is the lab identifier for junior-middle morality-and-law and senior-high ideological-political education. Product datasets must retain the curriculum name, textbook edition, province and exam year rather than flattening these variants.

## Chinese

### Scoring and writing candidates

| Source | Stage and size | Labels | Rights decision | Appropriate role |
| --- | --- | --- | --- | --- |
| [AES-Dataset](https://github.com/declan-haojin/AES-Dataset) | 300 high-school essays | Holistic score | `review_required`: dataset card says CC BY-NC 4.0 while the README license section says MIT; student contributor consent and relicensing authority are unclear | Small external calibration set only after written clearance |
| [Chinese Essay Dataset For Organization Evaluation](https://github.com/cnunlp/Chinese-Essay-Dataset-For-Organization-Evaluation) | 1,220 argumentative student essays; exact secondary stage not stated | Great/Medium/Bad organization plus sentence and paragraph functions | `review_required`: no license file | Organization and evidence-structure module after author license |
| [CCL 2023 CEFE](https://github.com/cubenlp/2023CCL_CEFE) | Real Chinese primary/junior-middle compositions; only small public training subsets | Error detection, correction and fluency grading | `review_required`: competition terms and commercial authorization required | Fluency/error evaluation after ECNU authorization |
| [CCL 2024 CEFE](https://github.com/cubenlp/2024CCL_CEFE) | Real primary/junior-middle exam compositions | Error detection, correction and fluency grading | `prohibited`: competition-only, commercial use prohibited | Licensing outreach only |
| [CCL 2025 CERRE](https://github.com/cubenlp/CERRE-2025CCL) | Real teaching-scene primary/junior-middle compositions | Rhetoric type, form and components | `prohibited`: competition-only, commercial use prohibited | Licensing outreach only |
| [Shanghai Gaokao Essays And Grades](https://modelscope.cn/datasets/MingRLY/College_Entrance_Exam_Chinese_Essays_and_Grades) | 86 high-school essays | Score and teacher comment | `review_required`: uploader claims Apache 2.0, but original publication and relicensing rights are unclear | Schema and feedback-format prototype only |
| [OpenNCEE Chinese Essay](https://huggingface.co/datasets/BabelTowerProject/OpenNCEE-Chinese-Essay) | Fewer than 1,000 Gaokao essays | Score/grade metadata | `prohibited`: CC BY-NC 4.0 is incompatible with product training | Noncommercial research or license outreach |

### Knowledge candidates

[M3KE](https://github.com/tjunlp-lab/M3KE) contains junior- and senior-high Chinese multiple-choice questions. [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) contains high-school objective and subjective questions with standard answers/analysis. They can evaluate subject competence and help draft Rubrics after provenance review, but neither contains authentic student responses with teacher partial-credit labels.

### Collection gap

Collect separate Gold sets for reading comprehension, classical Chinese, ancient poetry, language use, short answer and composition. Essays alone cannot validate short-answer grading. Every Gold answer needs question/version, structured Rubric, evidence spans, two independent teacher scores and adjudication.

## Mathematics

| Source | Stage and type | What it provides | What it does not provide | Decision |
| --- | --- | --- | --- | --- |
| [M3KE](https://github.com/tjunlp-lab/M3KE) | Junior/senior-high, four-option MCQ | Question, options, correct option | Student work, wrong-step diagnosis, partial credit | `review_required`; knowledge and rule evaluation only |
| [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) | Senior-high; part of 1,781 objective and 1,030 subjective questions across nine subjects | Question, standard answer and analysis | Authentic student steps and teacher scores | `review_required`; Rubric drafting and evaluation only |
| [Gaokao-LLM-data](https://huggingface.co/datasets/Interstellar174/Gaokao-LLM-data) | Senior-high open-ended math | Questions and generated reasoning | Real student work or teacher scoring | `review_required`; candidate Rubric generation only |

The required product data is handwritten or transcribed student work with line-level steps, misconception/error tags, final answer, units where applicable, point-by-point teacher awards and disagreement adjudication. Numeric-answer data should be routed to rules; LLM training should focus on step evidence and partial credit.

## English

| Source | Stage and size | Labels | Rights decision | Appropriate role |
| --- | --- | --- | --- | --- |
| [Chinese Student English Essay](https://huggingface.co/datasets/Xiaochr/Chinese-Student-English-Essay) | 13,270 Beijing high-school final-exam essays, two prompts | Overall, content, language and structure scores; Gaokao-aligned Rubric | `prohibited` for product use under CC BY-NC 4.0 | Best licensing target and noncommercial research benchmark |
| [Chinese Middle-School English Exam Questions](https://huggingface.co/datasets/dry-melon/Chinese-middle-school-English-exam-questions) | Grades 7-9; MCQ, cloze, reading and free response | Correct/reference answer | `review_required`: uploader claims CC BY 4.0; exam provenance must be verified | Rules, item routing and Rubric drafting only |
| [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) | Senior-high | Questions, standard answers and analysis | Student answers and teacher dimension scores | `review_required`; knowledge evaluation only |

The local copy of CSEE was structurally verified as 13,270 records, two prompt groups, 14,936,593 bytes, SHA256 `DDCA4267564CEEDD251B6F3FF366043590D5014F4E4BE53E74279CF4178D3E74`. It must remain outside product training until a commercial license is signed. The two-prompt design also makes random row splits unsafe; split by prompt/question group.

Additional Gold is needed for junior-middle writing, high-school continuation writing, translation, reading short answers and error correction. Scores should retain the exact regional/year Rubric because English writing standards vary by exam format.

## Physics

| Source | Stage and type | Allowed candidate role | Limitation |
| --- | --- | --- | --- |
| [M3KE](https://github.com/tjunlp-lab/M3KE) | Junior/senior-high MCQ | Knowledge and rule-grader evaluation after review | No student work or units/steps evidence |
| [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) | Senior-high objective and subjective questions | Rubric drafting and competence evaluation after review | Standard solution is not a population of student answers |
| [Gaokao-LLM-data](https://huggingface.co/datasets/Interstellar174/Gaokao-LLM-data) | Senior-high open-ended questions with generated reasoning | Generate candidate scoring points, then teacher-verify | Generated reasoning is not Gold and may contain errors |

Gold collection must cover calculations, unit mistakes, vector/direction mistakes, diagrams, experiment design, graph interpretation and alternative valid methods. Record each step score separately; a correct final value cannot imply correct reasoning.

## Chemistry

| Source | Stage and size | Allowed candidate role | Rights/provenance decision |
| --- | --- | --- | --- |
| [C-MHChem](https://huggingface.co/datasets/AI4Chem/C-MHChem) | 600 human-written junior/senior-high MCQs | Chemistry knowledge and rule evaluation | `review_required` despite MIT claim; verify authorship/source rights |
| [M3KE](https://github.com/tjunlp-lab/M3KE) | Junior/senior-high MCQ | Knowledge and rule evaluation | `review_required`; no student responses |
| [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) | Senior-high objective and subjective questions | Rubric drafting and evaluation | `review_required`; no student-scored responses |
| [Chinese High-School Chemistry Correction Dataset](https://huggingface.co/datasets/liushuaiqian/Chinese-High-School-Chemistry-Correction-Dataset) | 1K-10K instruction/QA records | Terminology warm-start and candidate Rubric generation | `review_required`: source description names textbooks and commercial teaching aids; Apache claim does not resolve those rights |

Gold collection must separately cover chemical equations, conditions, balancing, units, calculation steps, experiment phenomena, apparatus, safety and open-ended inference. Formula-equivalent expressions require a chemistry-aware canonicalizer before LLM judgment.

## Biology

[M3KE](https://github.com/tjunlp-lab/M3KE) provides junior/senior-high MCQ items, and [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) provides senior-high questions with reference answers. Both are knowledge resources, not grading data.

Gold collection must cover terminology, process ordering, causal mechanisms, genetics calculations, experimental controls, graph/table interpretation and acceptable synonymous expressions. Rubrics should distinguish a missing key term from a scientifically wrong causal claim.

## Politics

[M3KE](https://github.com/tjunlp-lab/M3KE) covers junior/senior-high politics MCQ, and [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) covers senior-high politics questions and reference answers. Neither contains student arguments with teacher evidence labels.

Gold records must include curriculum edition, province, exam year, policy/current-affairs validity window, material evidence, concept term, argument chain and teacher point allocation. Time-sensitive answers need versioned Rubrics and an expiry/review date.

## History

[M3KE](https://github.com/tjunlp-lab/M3KE) covers junior/senior-high history MCQ, while [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) adds senior-high source-based questions and reference analyses. Use only for knowledge evaluation/Rubric drafting after rights review.

Gold collection must represent chronology, historical context, cause/effect, comparison, source reliability and evidence-backed argument. Teacher labels should identify which quoted or paraphrased source evidence earned each point.

## Geography

[M3KE](https://github.com/tjunlp-lab/M3KE) covers junior/senior-high geography MCQ, and [GAOKAO-Bench](https://github.com/OpenLMLab/GAOKAO-Bench) includes senior-high questions with standard answers and analyses. No authentic student map/chart response corpus with teacher partial credit was found.

Gold collection must include maps, climate/terrain charts, spatial location, process chains, regional comparison and calculation. Keep image/OCR confidence and map-region references; text-only transcription loses grading evidence.

## Cross-Subject Sources

### M3KE

M3KE reports 20,477 four-option questions across 71 tasks and education levels from primary school through college. Its junior- and senior-high rows cover Chinese, history, politics, mathematics, physics, biology, chemistry and geography. It does not supply real student answers, teacher scores or partial-credit Rubrics.

### GAOKAO-Bench

GAOKAO-Bench reports 2,811 questions collected from 2010-2022 national Gaokao papers: 1,781 objective and 1,030 subjective. It covers Chinese, English, science/humanities mathematics, physics, chemistry, biology, politics, history and geography. The repository carries Apache 2.0, but product use still requires review of the underlying national-exam question rights. The included standard answer/analysis is useful for candidate Rubrics, not for learning student-error distributions.

## Acquisition Priority

1. Negotiate commercial rights for CSEE with its authors/data controller because it is the strongest immediately relevant Chinese-school scoring corpus.
2. Contact ECNU CCL CEFE/CERRE organizers for a separate commercial data license, consent/privacy documentation and allowed derivative-model terms.
3. Contact CNUNLP and AES-Dataset maintainers for author permission, contributor consent basis and commercial derivative-model rights.
4. Establish partner-school collection for the initial product scope. Public data cannot replace this step.
5. Conduct legal provenance review before importing M3KE, GAOKAO-Bench, C-MHChem, the middle-school English question bank or any dataset derived from textbooks/teaching aids.

## Product Gold Minimum

For each initial subject/question-type scope, collect at least 500 anonymized real responses spanning at least 10 distinct question plus Rubric groups. Every response must have two independent teacher annotations; disagreements require adjudication. A useful first release should not claim all-subject support merely because 500 answers exist in one subject.

Required fields are:

- Subject, grade, curriculum/textbook edition, province and exam year.
- Stable question ID, question assets, question type, maximum score and Rubric version.
- Anonymized answer text plus OCR confidence and image reference where permitted.
- Point-level teacher awards, answer evidence spans, missing points and error categories.
- Two independent scores, adjudicated score, annotator confidence and disagreement reason.
- Data controller, consent/legal basis, commercial training permission, retention and deletion terms.

Split train/validation/test by `question_id + rubric_version`, never by answer row. Hold out schools and exam periods for external validation where sample size permits.

## Training Gate

Do not start QLoRA merely because these public datasets have been downloaded. Fine-tuning can begin only when:

1. Commercial and derivative-model rights are documented for every training source.
2. The intended subject/question type has in-domain double-scored Gold data.
3. Privacy scanning and human review pass.
4. Question/Rubric grouped splits and cross-source text deduplication pass.
5. The prompt/rule baseline is measured on a frozen test set.
6. Fine-tuning improves score agreement, evidence validity and review recall without worsening subgroup bias or calibration.

Until those gates pass, permissive-looking question banks remain candidate knowledge resources, restricted real datasets remain research/licensing targets, and product status remains `NOT_READY`.
