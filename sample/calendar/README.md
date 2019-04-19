gcloud services list --available

gcloud beta iam roles list

gcloud services enable calendar-json.googleapis.com

gcloud iam service-accounts create gsuite-source

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:gsuite-source@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/owner
  
gcloud iam service-accounts keys create gsuite-source.json \
  --iam-account=gsuite-source@$PROJECT_ID.iam.gserviceaccount.com  

kubectl -n gsuite-sources create secret generic gsuite-source-key --from-file=key.json=gsuite-source.json --dry-run -o yaml | kubectl apply --filename -

kubectl -n default create secret generic gs-source-key --from-file=key.json=gsuite-source.json

Domain verification with Search

https://developers.google.com/calendar/v3/push

https://search.google.com/search-console/about?hl=en&utm_source=wmx&utm_medium=wmx-welcome

https://default.nachocano.org

openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout gsuite-private-key.pem -out gsuite-cert.pem

kubectl -n gsuite-sources create secret generic gsuite-private-key --from-file=key.pem=gsuite-private-key.pem --dry-run -o yaml | kubectl apply --filename -
kubectl -n default create secret generic gs-private-key --from-file=key.pem=gsuite-private-key.pem

kubectl -n gsuite-sources create secret generic gsuite-cert-key --from-file=cert.pem=gsuite-cert.pem --dry-run -o yaml | kubectl apply --filename -
kubectl -n default create secret generic gs-cert-key --from-file=cert.pem=gsuite-cert.pem
