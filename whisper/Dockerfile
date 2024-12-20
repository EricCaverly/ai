# Use an official Python base image
FROM python:3.10-slim

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# install FFMPeg
RUN apt-get -y update
RUN apt-get -y install ffmpeg

# Set the working directory
WORKDIR /app

# Copy application files
COPY . .

# Volume for models
VOLUME /root/.cache/whisper

# Install Python dependencies
RUN pip install --no-cache-dir fastapi uvicorn python-multipart openai-whisper

# Expose the port your API will run on
EXPOSE 8000

# Command to run the application
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
