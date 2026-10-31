FROM python:3.13-slim
WORKDIR /tests
COPY requirements.txt .
RUN pip install --no-cache-dir --root-user-action=ignore -r requirements.txt
COPY *.py .
ENTRYPOINT ["python", "-m", "pytest", "-v", "-p", "no:cacheprovider"]
