import pandas as pd
from sklearn.ensemble import IsolationForest
import pickle
import sys
import json

# Updated to detect anomalies for each metric type separately

def load_data(csv_file):
    df = pd.read_csv(csv_file)
    # Pivot to get temperature and humidity as separate time series
    df_pivot = df.pivot_table(index='timestamp', columns='metric_type', values='value', aggfunc='first').reset_index()
    # Ensure columns exist, fill NaNs with forward/backward fill or interpolation
    df_pivot = df_pivot[['timestamp', 'temperature_2m', 'relative_humidity_2m', 'precipitation', 'wind_speed_10m', 'dew_point_2m']].ffill().bfill().fillna(0)
    return df_pivot

def train_and_predict_anomalies_per_metric(df):
    """Train separate models for each metric type and detect anomalies"""
    results = {}

    metric_types = ['temperature_2m', 'relative_humidity_2m', 'precipitation', 'wind_speed_10m', 'dew_point_2m']

    for metric_type in metric_types:
        if metric_type not in df.columns:
            continue

        # Get data for this metric
        metric_data = df[['timestamp', metric_type]].dropna()
        if len(metric_data) < 10:
            continue

        # Prepare features (just the single metric value)
        features = metric_data[[metric_type]]

        # Train Isolation Forest model
        model = IsolationForest(contamination=0.05, random_state=42, n_estimators=100)
        model.fit(features)

        # Get predictions and scores
        predictions = model.predict(features)  # -1 for anomaly, 1 for normal
        scores = model.decision_function(features)  # Anomaly scores (more negative = more anomalous)

        # Find anomalies
        anomalies = []
        for idx, row in metric_data.iterrows():
            if predictions[idx] == -1:  # This is an anomaly
                anomaly = {
                    "timestamp": row['timestamp'],
                    "metric_type": metric_type,
                    "value": float(row[metric_type]),
                    "anomaly_score": float(scores[idx]),
                    "severity": calculate_severity(scores[idx])
                }
                anomalies.append(anomaly)

        results[metric_type] = {
            "model": model,
            "anomalies": anomalies,
            "total_points": len(metric_data),
            "anomalies_found": len(anomalies)
        }

    return results

def calculate_severity(anomaly_score):
    """Calculate severity based on anomaly score"""
    abs_score = abs(anomaly_score)
    if abs_score > 0.15:
        return "high"
    elif abs_score > 0.1:
        return "medium"
    else:
        return "low"

def save_models(results):
    """Save trained models to disk"""
    for metric_type, result in results.items():
        model_filename = f"model_{metric_type}.pkl"
        with open(model_filename, "wb") as f:
            pickle.dump(result["model"], f)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python train.py <csv_file>")
        sys.exit(1)

    csv_file = sys.argv[1]
    df = load_data(csv_file)
    results = train_and_predict_anomalies_per_metric(df)

    # Save models
    save_models(results)

    # Collect all anomalies
    all_anomalies = []
    for metric_type, result in results.items():
        all_anomalies.extend(result["anomalies"])

    # Output JSON to stdout (Go will capture this)
    output = {
        "models_saved": len(results),
        "total_anomalies_found": len(all_anomalies),
        "anomalies": all_anomalies,
        "metrics_processed": list(results.keys())
    }

    print(json.dumps(output))
