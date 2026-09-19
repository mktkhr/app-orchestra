# The five axes are not one number, and the retriever swap proves it

Measured 2026-09-19. Local models and the local embedding stack only, no
cost.

## What was tried

`retrieval-ceiling-2026-09-19.md` found axis D's binding constraint in the
first-stage retriever: six of twenty-five answers never retrieved, K=50 no
better than K=20. So the retriever itself became the candidate for change.
Six embedding configurations, measured on the extension set through the
product's own retrieval shape (examples embedded and reranked):

| embedding                     | axis D    | axis E    |
| ----------------------------- | --------- | --------- |
| **bge-m3-q8**                 | **21/25** | 22/25     |
| ruri-v3-310m-q8-mean          | 20/25     | **23/25** |
| e5-large-q8 (shipped)         | 19/25     | 22/25     |
| qwen3-embedding-0.6b-q8-plain | 16/25     | 23/25     |
| qwen3-embedding-0.6b-q8       | 15/25     | 23/25     |
| ruri-v3-310m-q8               | 12/25     | 16/25     |

The shipped retriever is not the best available on these axes. Worth
noting in passing: `ruri-v3-310m-q8` and `ruri-v3-310m-q8-mean` are the
same weights with different pooling, and differ by **eight axis-D rows** -
a configuration detail, not a model choice.

## Swapping it in: +4 everywhere, then -3 and -2

End to end with `ORCHESTRA_NARROWING_EMBED_MODEL=bge-m3-q8`, extension
set (axes D and E only):

| model           | shipped | bge-m3 | axis D      | axis E      |
| --------------- | ------- | ------ | ----------- | ----------- |
| `bonsai2-27b`   | 60      | **64** | 44 → 48     | 76 → 80     |
| `gemma4-12b-q8` | 54      | **58** | 40 → **52** | 68 → 64     |
| `qwen3.5-9b-q8` | 50      | **54** | 40 → 36     | 60 → **72** |

Three models, +4 each - and unlike a wording, the sign does not flip per
model. That looked like the best-value change of the day.

Then the same swap on the original 100 questions, which carry axes A, B
and C as well:

| model           | shipped | bge-m3 | A           | B           | C       | D       | E           |
| --------------- | ------- | ------ | ----------- | ----------- | ------- | ------- | ----------- |
| `qwen3.5-9b-q8` | **77**  | 74     | 92 → **84** | 80 → 88     | 64 → 68 | 60 → 53 | 90 → **60** |
| `bonsai2-27b`   | **81**  | 79     | 92 → **84** | 92 → **96** | 60 → 64 | 73 → 66 | 90 → 80     |

**Both models lose.** The 50-question set said +4; the 100-question set
says -3 and -2. The swap is not adopted.

## Why the two sets disagree, and what that says about the score

The extension set is axes D and E only - it was built that way on purpose,
to raise the resolution of the two axes that separate models. It is
therefore **not a sample of the task**. `bge-m3-q8` buys axis D and pays
for it on axis A, and the extension set cannot see axis A at all.

That is the specific lesson. The general one is larger: **the five axes
are different kinds of failure, and adding them into one number averages
things that are not comparable.** What a miss costs the person asking:

| axis                                   | what a miss looks like                                     | can the person recover?                         |
| -------------------------------------- | ---------------------------------------------------------- | ----------------------------------------------- |
| **A** (verb carries no selectivity)    | a plausible table of the wrong resource                    | **no** - nothing signals the error              |
| **B** (same noun in two services)      | 受注 answered with 発注; often the right answer was to ask | **no** - the work proceeds on the wrong record  |
| **C** (near neighbours in one service) | 在庫品目 answered with 在庫ロット                          | hard - the two look alike                       |
| **D** (vocabulary gap)                 | a wrong operation, or none                                 | **yes** - rephrase, or take an alternative chip |
| **E** (a decoy closer than the answer) | a settings value instead of records                        | **yes** - obviously not what was asked          |

A and B are the expensive ones: the answer looks right. D and E are
recoverable, and the product already has the machinery - `alternatives`
("違いましたか？"), measured as `correct@shown`, and the offline finding
that reordering those chips by a probability moves `correct@shown` 79 → 85
(`jev-chip-order.md`).

So `bge-m3-q8` trades eight points of axis A for seven of axis D on
`qwen3.5-9b-q8`, and those eight are worth more than those seven. The
totals (-3) understate how bad the trade is; the axis table shows it.

## How to read these numbers from here

1. **Read the axes, not the sum.** A total is only comparable between two
   runs of the _same_ question set with the same axis mix.
2. **A and B first.** They are the failures the person cannot catch.
3. **D and E are partly recoverable** by the alternatives chips, so a loss
   there costs less than the same loss on A.
4. **The extension set (D/E only) is a magnifier, not a scoreboard.** Use
   it to see a difference clearly; confirm on the 100 before believing it.

## Where `bonsai2-27b` stands, axis by axis

|                           | A   | B      | C   | D      | E   | total  |
| ------------------------- | --- | ------ | --- | ------ | --- | ------ |
| `qwen3.5-9b-q8` (shipped) | 92  | 80     | 64  | 60     | 90  | 77     |
| `bonsai2-27b`             | 92  | **92** | 60  | **73** | 90  | **81** |

Equal on A, **twelve points better on B** - the cross-service homonyms,
the second of the two expensive axes - thirteen better on D, four worse on
C. On the reading above that is a better profile than the totals alone
suggest, and it is the reason to keep working on this model rather than
the incumbent.

## Raw data

Scratchpad `recall-embed.ts` (the six-configuration sweep);
`e2e/shortlist/out/{on,ext}-default-stages2*-embedbge-m3-q8.{jsonl,log}`
for the end-to-end runs.
