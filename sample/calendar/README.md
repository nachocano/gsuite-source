gcloud services list --available
gcloud beta iam roles list

gcloud services enable calendar-json.googleapis.com

gcloud iam service-accounts create gsuite-source

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:gsuite-source@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/owner
  
gcloud iam service-accounts keys create gsuite-source.json \
  --iam-account=gsuite-source@$PROJECT_ID.iam.gserviceaccount.com  

