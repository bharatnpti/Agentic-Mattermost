1. ./mvn clean install
2. docker build -t mattermost-app .
3. export OPENAI_API_KEY=your_openai_api_key_here
4. ./docker-run.sh start prod


To Stop -

./docker-run.sh clean