function mean(values) {
  return values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : 0;
}

function ocrBand(value) {
  return value < 0.85 ? "low" : "high";
}

export function evaluateOperationalSlices(predictions, minimumRecords = 1) {
  const dimensions = {
    subject: (item) => item.observation.subject,
    question_type: (item) => item.observation.question_type,
    ocr_band: (item) => ocrBand(item.observation.ocr_confidence),
    answer_length_band: (item) => item.observation.answer_length_band
  };
  const slices = {};
  const disparities = {};
  const insufficient = [];
  for (const [dimension, keyOf] of Object.entries(dimensions)) {
    const groups = new Map();
    for (const prediction of predictions) {
      const key = keyOf(prediction) ?? "unknown";
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(prediction);
    }
    slices[dimension] = [...groups.entries()].map(([value, records]) => {
      const normalizedErrors = records.map((item) => Math.abs(item.observation.model_score - item.observation.gold_score) / item.observation.max_score);
      const overgrades = records.map((item) => item.observation.model_score > item.observation.gold_score ? 1 : 0);
      const undergrades = records.map((item) => item.observation.model_score < item.observation.gold_score ? 1 : 0);
      const result = {
        value,
        count: records.length,
        normalized_mae: mean(normalizedErrors),
        acceptable_agreement_rate: mean(records.map((item) => item.outcome)),
        overgrade_rate: mean(overgrades),
        undergrade_rate: mean(undergrades),
        mean_calibrated_probability: mean(records.map((item) => item.probability))
      };
      if (records.length < minimumRecords) insufficient.push({ dimension, value, count: records.length, required: minimumRecords });
      return result;
    });
    const eligible = slices[dimension].filter((slice) => slice.count >= minimumRecords);
    const nmaes = eligible.map((slice) => slice.normalized_mae);
    disparities[dimension] = { eligible_slices: eligible.length, normalized_mae_gap: nmaes.length >= 2 ? Math.max(...nmaes) - Math.min(...nmaes) : null };
  }
  return { dimensions: slices, disparities, insufficient_slices: insufficient };
}

export function checkCalibrationFairnessGate(calibration, fairness, level, config) {
  const gate = config[level];
  if (!gate) throw new Error(`Unknown calibration gate: ${level}`);
  const reasons = [];
  if (calibration.evaluation_records < gate.minimum_evaluation_records) reasons.push(`evaluation_records ${calibration.evaluation_records} < ${gate.minimum_evaluation_records}`);
  if (calibration.ece > gate.maximum_ece) reasons.push(`ece ${calibration.ece} > ${gate.maximum_ece}`);
  if (calibration.brier_score > gate.maximum_brier_score) reasons.push(`brier_score ${calibration.brier_score} > ${gate.maximum_brier_score}`);
  if (fairness.insufficient_slices.length) reasons.push(`${fairness.insufficient_slices.length} slices are below minimum_records_per_slice ${gate.minimum_records_per_slice}`);
  for (const [dimension, disparity] of Object.entries(fairness.disparities)) {
    if (disparity.normalized_mae_gap !== null && disparity.normalized_mae_gap > gate.maximum_slice_nmae_gap) {
      reasons.push(`${dimension}.normalized_mae_gap ${disparity.normalized_mae_gap} > ${gate.maximum_slice_nmae_gap}`);
    }
  }
  return { level, passed: reasons.length === 0, reasons };
}
