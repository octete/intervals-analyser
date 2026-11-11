# Interpreting Pedal Balance Lab Results

This guide explains what the application calculates, how to read each result, and what conclusions the data can and cannot support.

## The short version

The most useful question is usually not “Am I exactly 50/50?” It is:

> Is the left/right pattern repeatable, and does it change systematically with power or cadence?

Look for patterns that recur across comparable rides rather than reacting to one number from one activity. A stable 49/51 split may simply be your normal pattern. A balance that moves consistently with power, cadence, fatigue, position, or pain is potentially more informative.

This software is an exploratory analysis tool. It does not diagnose an injury, establish that one leg is weak, or prescribe a bike-fit change. Power-meter accuracy, recording behaviour, fatigue, terrain, riding position, and normal biological variation all affect the result.

## What a balance value means

A value such as `48.5 / 51.5` means:

- the left side contributed 48.5% of the measured total;
- the right side contributed 51.5% of the measured total.

The two values always add to 100%. Balance is a share of total measured power, not a direct measurement of strength, muscle activation, joint loading, or pedalling technique.

The FIT format stores a contribution value and a flag identifying it as the right-side contribution. The application converts this into left/right form for display. Records without valid power, cadence, timestamp, and explicitly right-labelled balance are skipped rather than guessed.

## Reported versus corrected balance

The application shows two versions of the analysis.

### Reported

**Reported balance** is the value recorded in the FIT file by the Wahoo POWRLINK ZERO system. It is the result you would normally see on a head unit or training platform.

Use this view when you want to answer questions such as:

- What did the power meter report?
- Is the reported split stable across power or cadence?
- Does the same pattern recur from ride to ride?

### Corrected

**Corrected balance** estimates the split after accounting for the unequal physical crank lengths. For a reported left contribution \(L_r\), left crank length \(l\), and right crank length \(r\), the application calculates:

```text
corrected left % = 100 × (Lr × l) / ((Lr × l) + ((100 − Lr) × r))
corrected right % = 100 − corrected left %
```

With 160 mm on the left and 165 mm on the right, a reported `50.0 / 50.0` becomes approximately:

```text
49.2 / 50.8 corrected
```

This happens because the same reported contribution is scaled by a shorter lever on the left and a longer lever on the right.

The power meter's configured 162.5 mm length is a common scaling denominator. It matters when estimating corrected absolute left and right watts, but it cancels out when calculating their percentage split. Therefore changing only the configured length does not change the corrected L/R percentage currently displayed by this application.

The correction is a mathematical estimate, not an independent measurement. It assumes that crank-length scaling is the relevant systematic difference and that the recorded side contributions are otherwise comparable. It cannot correct sensor calibration errors, installation differences, temperature drift, drivetrain effects, or physiological adaptation to unequal cranks.

## Samples, represented time, and skipped records

### Valid samples

A sample is one FIT record containing all of the following:

- a valid timestamp;
- total power;
- cadence;
- dual-sided left/right balance with the FIT right-side flag set.

The sample count tells you how many such records contributed to the analysis. If the device records once per second, 600 samples will often represent about ten minutes, but the application does not assume a fixed recording interval.

### Represented time

Each sample is assigned the time until the following valid timestamp. Normal positive intervals up to 30 seconds are used directly. Invalid intervals, long gaps, and the final sample use the median normal recording interval as a fallback.

Represented time is therefore an estimate of the recorded time covered by valid samples. It deliberately avoids treating a long pause or recording gap as continuous pedalling.

### Skipped records

Skipped records were present in the FIT activity but lacked one or more required values, used an invalid FIT sentinel, or did not explicitly identify the balance value as right-side data.

A high skipped count is worth investigating. It can indicate recording dropouts, a device that does not write balance on every record, or a FIT file whose balance field is encoded differently.

## Power buckets

Power buckets group samples by total recorded power. With a 50 W bucket size:

```text
0–50 W
50–100 W
100–150 W
150–200 W
...
```

The lower boundary is included and the upper boundary is excluded. A sample at exactly 50 W belongs to the `50–100 W` bucket.

Buckets make it easier to see whether balance changes with intensity. For example:

```text
100–150 W: 49.8 / 50.2
200–250 W: 49.1 / 50.9
300–350 W: 47.9 / 52.1
```

That pattern suggests an increasing right-side share as power rises. It does not, by itself, explain why.

### Samples

The number of valid records in the bucket. More samples usually make the summary more stable, although many adjacent samples from one continuous effort are not the same as many independent observations.

### Time

The estimated duration represented by those samples. Use it alongside sample count: a bucket based on only a few seconds should not carry the same weight as one containing several sustained efforts.

### Mean power

The arithmetic mean of the recorded total-power samples in the bucket. It confirms where the observations actually lie inside the bucket.

### Mean L/R balance

The arithmetic mean of all balance samples in the bucket. Every sample receives equal weight; the mean is not explicitly duration-weighted.

The mean is useful for the overall direction and size of the split, but it can be influenced by brief extreme values.

### Median L/R balance

The middle balance value after sorting the samples. It is less sensitive than the mean to occasional spikes or dropouts.

Compare mean and median:

- similar values suggest a reasonably symmetric distribution around the centre;
- a noticeably different mean and median suggest skew or outliers;
- neither tells you whether the imbalance was continuous or occurred in separate efforts.

### Standard deviation

Standard deviation describes sample-to-sample spread around the mean and is shown in **percentage points**.

For example, a mean left contribution of 49.0 with a standard deviation of 0.6 means the balance was fairly tightly clustered. A standard deviation of 3.0 means it varied much more.

High variability can reflect real changes, transitions between seated and standing riding, acceleration, low-power noise, rough terrain, recording artefacts, or a mixture of different effort types. It is not automatically evidence of unstable pedalling.

The application uses population standard deviation for the samples contained in the bucket.

### Minimum samples and minimum time

Buckets are hidden unless they satisfy both configured thresholds. Raising these thresholds removes sparsely populated results and is often helpful when looking for robust patterns.

A good workflow is to begin with the defaults, then increase the thresholds and see whether the apparent pattern survives. If it disappears immediately, it was probably supported by too little data.

## Power × cadence heatmap

The heatmap divides the ride along two dimensions:

- total power buckets;
- cadence buckets.

For example, one tile might represent `200–250 W` at `80–90 rpm`. The tile displays the mean actual split, such as `48.7 / 51.3`, for samples in that combination.

This helps separate two effects that a one-dimensional power table can mix together. Higher-power riding may also happen at a different cadence, so an apparent “power effect” could partly be a cadence effect.

Use the heatmap to ask:

- At a similar cadence, does balance change as power rises?
- At similar power, does balance change as cadence rises?
- Is an apparent imbalance widespread or confined to one small region?
- Does the corrected view preserve the same pattern as the reported view?

The colour encodes the direction and magnitude of departure from 50/50; the printed L/R number is the value to rely on. Stronger colour means farther from 50/50, not necessarily greater confidence. Confidence still depends on the cell's sample count and duration.

As with power buckets, cadence boundaries are lower-inclusive and upper-exclusive. Exactly 80 rpm belongs to the `80–90 rpm` bucket when the bucket size is 10 rpm.

Low-power and zero-cadence cells deserve particular caution. The general bucket and heatmap analysis includes valid records at zero watts or zero rpm, whereas the regression excludes very low power and cadence. Coasting, starts, stops, and sensor noise can dominate those cells without saying much about active pedalling asymmetry.

## The regression model

The regression estimates how left-side percentage changes with power and cadence while statistically holding the other variable constant.

The implemented model is:

```text
predicted left % = a
                 + b × ((power − 200 W) / 100 W)
                 + c × ((cadence − 85 rpm) / 10 rpm)
```

This is the centred form of `L = a + bP + cC`. Centring makes the coefficients easier to interpret.

By default, the regression uses samples at or above 50 W and 40 rpm. This reduces the influence of coasting and very-low-load data. The fit is ordinary unweighted least squares: each qualifying sample contributes equally.

### Baseline or intercept (`a`)

The displayed expected balance at 200 W and 85 rpm is the intercept.

Example:

```text
Expected balance: 48.9 / 51.1
```

The model predicts a 48.9% left contribution at the reference condition of 200 W and 85 rpm.

This estimate is most trustworthy when the activity contains substantial data near 200 W and 85 rpm. If those reference values lie outside or near the edge of the observed data, the intercept is partly an extrapolation.

### Power effect (`b`)

The power coefficient is shown in percentage points per 100 W, with cadence held constant.

Example:

```text
Power effect: −0.70 pp / 100 W
```

For every additional 100 W, the model predicts that the left contribution falls by 0.70 percentage points. At the same cadence, the predicted split would move approximately from:

```text
200 W: 48.9 / 51.1
300 W: 48.2 / 51.8
```

A positive coefficient means the predicted left share rises with power. A negative coefficient means the predicted right share rises.

### Cadence effect (`c`)

The cadence coefficient is shown in percentage points per 10 rpm, with power held constant.

Example:

```text
Cadence effect: +0.25 pp / 10 rpm
```

For every additional 10 rpm, the model predicts that the left contribution rises by 0.25 percentage points at the same power.

### Combining coefficients

Suppose the model reports:

```text
a = 48.9% left at 200 W and 85 rpm
b = −0.70 pp per 100 W
c = +0.25 pp per 10 rpm
```

At 300 W and 95 rpm:

```text
predicted left = 48.9 + (−0.70 × 1) + (+0.25 × 1)
               = 48.45%
predicted L/R  = 48.45 / 51.55
```

This is a model prediction, not necessarily an observed heatmap cell.

### R²

R² describes how much of the sample-to-sample variation in left percentage is explained by the linear power and cadence terms in this activity.

- `R² = 0.00`: the model explains essentially none of the variation;
- `R² = 0.25`: it explains about 25% of the observed variation;
- `R² = 1.00`: it reproduces all observed values exactly.

A low R² does not prove that the coefficients are meaningless, but it means power and cadence alone are poor predictors of moment-to-moment balance. Other factors or measurement noise dominate. A high R² describes fit to this dataset; it does not prove causation or guarantee that the relationship will repeat in another ride.

### RMSE

RMSE is the typical size of a model prediction error, in percentage points of left balance.

For example, `RMSE = 1.2 pp` means observed left balance values typically differ from model predictions by roughly 1.2 percentage points. Lower is a tighter fit.

Judge a coefficient against RMSE and repeatability. A very small power effect combined with large RMSE and inconsistent results across rides is weak evidence of a meaningful relationship.

### Reported and corrected regressions

The reported regression models the meter's recorded balance. The corrected regression applies the crank-length correction to each sample before fitting.

Because the crank correction is a smooth transformation of the same underlying samples, the two models are not independent evidence. Compare them to understand the impact of the assumed crank-length correction, not as two separate experiments.

### What this regression does not include

The current model does not include:

- a power-by-cadence interaction term;
- fatigue or elapsed time;
- seated versus standing position;
- gradient, speed, torque, or pedal phase;
- separate random effects for intervals or rides;
- confidence intervals, p-values, or hypothesis tests;
- weighting for sample duration or serial correlation.

Adjacent one-second samples are strongly related to one another, so a large sample count should not be interpreted as the same number of independent experiments. The regression is best treated as a descriptive summary.

## A practical interpretation workflow

### 1. Check data quality

Confirm that valid samples and represented time are plausible for the ride. Investigate a very high skipped count or unexpectedly short represented time before interpreting balance.

### 2. Ignore non-pedalling regions first

Begin with sustained pedalling regions rather than the `0–50 W` bucket or zero/very-low-cadence heatmap cells.

### 3. Look for coverage

Prefer buckets and cells with meaningful time and repeated exposure. Suppress thin regions using the minimum-sample and minimum-time settings.

### 4. Compare mean, median, and variability

A consistent offset with low spread is different from an average produced by highly variable samples. Mean/median disagreement is a prompt to inspect the distribution or ride context.

### 5. Read the heatmap before the regression

Check whether the apparent power and cadence relationships are visible across multiple populated cells. A coefficient driven by one extreme or sparsely populated region is less convincing.

### 6. Interpret the regression in context

Read the baseline, power effect, cadence effect, R², and RMSE together. Do not treat the sign of a coefficient alone as a conclusion.

### 7. Compare reported and corrected views

The difference between them is the consequence of the crank-length assumption. Focus on whether the pattern across effort levels changes, not only on which value is closer to 50/50.

### 8. Repeat under comparable conditions

Compare multiple rides or controlled intervals with similar:

- power targets;
- cadence targets;
- seated/standing position;
- terrain or trainer mode;
- fatigue state;
- power-meter calibration and firmware;
- bike setup.

A pattern that repeats is more credible than an isolated result.

## Common patterns and cautious interpretations

### Stable offset across the whole range

Example: most populated regions sit near `49/51`, with small standard deviations and near-zero regression slopes.

Interpretation: a consistent side preference or systematic measurement/setup offset is more likely than an intensity-specific change. This may simply be the rider's repeatable baseline.

### Imbalance grows with power

Example: the left share falls across successively higher power buckets, heatmap rows show the same trend at similar cadence, and the regression power coefficient is negative across several rides.

Interpretation: there is descriptive evidence that the right share increases with load. Possible explanations include fatigue, strength or coordination differences, position changes, sensor behaviour, or the way high-power efforts were performed. The data does not distinguish these causes.

### Imbalance changes mainly with cadence

Example: the heatmap changes across cadence at similar power and the cadence coefficient is repeatably non-zero, while the power coefficient is small.

Interpretation: cadence is more strongly associated with the observed balance in these rides. This is still association, not proof that cadence causes the change.

### Large variation with weak regression fit

Example: standard deviations and RMSE are high, R² is low, and neighbouring heatmap cells are inconsistent.

Interpretation: power and cadence do not provide a stable explanation. Mixed riding conditions, transitions, noise, or omitted factors may dominate. Avoid drawing a strong asymmetry conclusion.

### One isolated extreme cell

Interpretation: check sample count, duration, and ride context. A sprint, standing acceleration, dropout, or short transition can create an extreme tile. Look for repetition before treating it as a pattern.

## What not to conclude

These results alone cannot establish:

- that one leg is injured, weak, or neurologically impaired;
- that 50/50 is the ideal target for you;
- that changing crank length will improve performance or prevent injury;
- that a statistical association is causal;
- that a tiny difference is practically meaningful;
- that the corrected estimate is “true” and the reported value is “wrong.”

If the analysis accompanies pain, a sudden persistent change, rehabilitation, or a major bike-fit decision, combine it with repeatable controlled testing and advice from an appropriately qualified clinician, physiotherapist, coach, or bike fitter.

## Suggested controlled test

For cleaner comparisons, perform several steady blocks after warming up, for example:

1. Hold one cadence while completing blocks at several power levels.
2. Hold one power level while completing blocks at several cadences.
3. Stay seated unless position is the variable being tested.
4. Keep each block long enough to populate its bucket or heatmap cell.
5. Repeat the protocol on another day.

This creates better separation between power and cadence than a variable outdoor ride and makes the regression coefficients easier to interpret.
