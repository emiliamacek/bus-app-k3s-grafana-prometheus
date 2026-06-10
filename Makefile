up:
	bash setup.sh

down:
	docker compose down -v

rebuild:
	docker build -t localhost:5000/bus-app:latest .
	docker push localhost:5000/bus-app:latest
	docker exec k3s-cluster kubectl rollout restart deployment bus-app

logs:
	docker exec k3s-cluster kubectl logs deployment/bus-app -f

pods:
	docker exec k3s-cluster kubectl get pods -A

services:
	docker exec k3s-cluster kubectl get svc -A
