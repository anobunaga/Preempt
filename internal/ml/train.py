import redis
import json
import pandas as pd
import numpy as np
from sklearn.ensemble import IsolationForest
import pickle
import time
import os

def connect_redis():
    """Connect to Redis"""
    r = redis.Redis(host='localhost', port=6379, db=0, decode_responses=True)
    return r

def train_and_detect(metrics_data, location, job_id):
    """Train Isolation Forest model and detect anomalies"""
    
    # Create models directory if it doesn't exist
    os.makedirs('ml_models', exist_ok=True)
    
    # Convert to DataFrame
    df = pd.DataFrame(metrics_data)
    df['timestamp'] = pd.to_datetime(df['timestamp'])
    
    anomalies = []
    models_saved = 0
    metrics_processed = []
    
    # Process each metric type separately
    for metric_type in df['metric_type'].unique():
        metric_df = df[df['metric_type'] == metric_type].copy()
        
        if len(metric_df) < 10:
            continue
            
        # Prepare data
        X = metric_df[['value']].values
        
        # Train Isolation Forest
        model = IsolationForest(
            contamination=0.05,
            random_state=42,
            n_estimators=100
        )
        predictions = model.fit_predict(X)
        scores = model.score_samples(X)
        
        # Save model
        model_filename = f'ml_models/{metric_type}_model.pkl'
        try:
            with open(model_filename, 'wb') as f:
                pickle.dump(model, f)
            models_saved += 1
            metrics_processed.append(metric_type)
        except Exception as e:
            print(f"Warning: Could not save model for {metric_type}: {e}")
        
        # Find anomalies (prediction == -1)
        anomaly_indices = np.where(predictions == -1)[0]
        
        for idx in anomaly_indices:
            row = metric_df.iloc[idx]
            anomaly_score = abs(scores[idx])
            
            # Determine severity based on anomaly score
            if anomaly_score > 0.5:
                severity = "high"
            elif anomaly_score > 0.3:
                severity = "medium"
            else:
                severity = "low"
            
            anomalies.append({
                'timestamp': row['timestamp'].isoformat(),
                'metric_type': metric_type,
                'value': float(row['value']),
                'anomaly_score': float(anomaly_score),
                'severity': severity
            })
    
    result = {
        'job_id': job_id,
        'location': location,
        'models_saved': models_saved,
        'total_anomalies_found': len(anomalies),
        'anomalies': anomalies,
        'metrics_processed': metrics_processed
    }
    
    return result

def main():
    """Main function to process ML jobs from Redis"""
    r = connect_redis()
    
    # Create consumer group if it doesn't exist
    try:
        r.xgroup_create('ml_input', 'ml_workers', id='0', mkstream=True)
    except redis.exceptions.ResponseError as e:
        if "BUSYGROUP" not in str(e):
            print(f"Error creating consumer group: {e}")
    
    print("ML worker started, listening for jobs on ml_input stream...")
    
    while True:
        try:
            # Read from stream with consumer group
            messages = r.xreadgroup(
                'ml_workers',
                'worker-1',
                {'ml_input': '>'},
                count=1,
                block=5000
            )
            
            if not messages:
                continue
                
            for stream_name, stream_messages in messages:
                for message_id, message_data in stream_messages:
                    try:
                        # Parse the data
                        data_str = message_data['data']
                        payload = json.loads(data_str)
                        
                        location = payload['location']
                        metrics_data = payload['metrics']
                        job_id = payload['job_id']
                        
                        print(f"Processing ML job {job_id} for location {location} with {len(metrics_data)} metrics")
                        
                        # Train and detect
                        result = train_and_detect(metrics_data, location, job_id)
                        
                        # Publish results to output stream
                        r.xadd('ml_output', {'data': json.dumps(result)})
                        
                        # Acknowledge the message
                        r.xack('ml_input', 'ml_workers', message_id)
                        
                        print(f"Job {job_id} completed: {result['total_anomalies_found']} anomalies found")
                        
                    except Exception as e:
                        print(f"Error processing message {message_id}: {e}")
                        continue
                        
        except KeyboardInterrupt:
            print("\nShutting down ML worker...")
            break
        except Exception as e:
            print(f"Error in main loop: {e}")
            time.sleep(1)

if __name__ == "__main__":
    main()
