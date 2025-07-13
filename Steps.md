1. ./mvn clean install
2. docker build -t mattermost-app .
3. export OPENAI_API_KEY=your_openai_api_key_here
4. ./docker-run.sh start prod
5. Open mattermost ui -http://localhost:8065
6. Upload plugin file to mattermost - [agentic-mattermost-plugin-0.5.0+19b5932.tar.gz](agentic-mattermost-plugin-0.5.0%2B19b5932.tar.gz)
7. Go to System Console - ![img.png](img.png)
8. Go to Plugin Management - ![img_1.png](img_1.png)
9. Upload file mentioned in Step 6 - ![img_2.png](img_2.png)
10. Enable Plugin - ![img_3.png](img_3.png)

To Stop -

./docker-run.sh clean