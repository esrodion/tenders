## Сервис для публикации тендеров и предложений

Для запуска сервиса при помощи Docker:
```
docker build --tag 'tenders' . 
docker run -it 'tenders'
```

Для корректного запуска требуется указать переменную окружения POSTGRES_CONN - URL для подключения к postgres.  
Сервис предполагает, что в базе данных уже созданы и заполнены таблицы employee, organization, organization_responsible. 



