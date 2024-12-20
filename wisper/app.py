from fastapi import FastAPI, UploadFile
import whisper

app = FastAPI()

# Load the Whisper model (e.g., "base", "small", "large")
model = whisper.load_model("base")

@app.post("/transcribe/")
async def transcribe(file: UploadFile):
    audio_file = f"temp_{file.filename}"
    
    # Save uploaded file to disk
    with open(audio_file, "wb") as buffer:
        buffer.write(await file.read())

    # Transcribe the audio
    result = model.transcribe(audio_file)
    
    # Remove temporary file
    import os
    os.remove(audio_file)

    return {"transcription": result["text"]}
