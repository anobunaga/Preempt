import pandas as pd
from sklearn.ensemble import IsolationForest
import pickle
import mysql.connector

#example isolation forest training script (can change model later)

def load_data():
    db = mysql.connector.connect(
        host="localhost", user="root", password="pass", database="preempt"
    )
    df = pd.read_sql("SELECT ts, temperature, humidity FROM metrics ORDER BY ts DESC LIMIT 7*24", db)
    return df

def train_model(df):
    features = df[["temperature", "humidity"]]
    model = IsolationForest(contamination=0.02)
    model.fit(features)
    return model

if __name__ == "__main__":
    df = load_data()
    model = train_model(df)

    with open("model.pkl", "wb") as f:
        pickle.dump(model, f)

    print("Model trained and saved to model.pkl")