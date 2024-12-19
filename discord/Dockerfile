FROM python:3.14.0a3-bullseye

RUN apt-get install -y gcc

WORKDIR /app

COPY . .

RUN pip install --no-cache-dir -r pip_list

CMD ["python", "main.py"]
