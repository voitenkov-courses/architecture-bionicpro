from airflow import DAG
from airflow.operators.python import PythonOperator
from datetime import datetime, timedelta

CH_HOST = 'olap_db'
CH_PORT = 9000
CRM_HOST = 'crm_db'
CRM_PORT = 5432
CRM_DB = 'crm_db'
CRM_USER = 'crm_user'
CRM_PASSWORD = 'crm_password'

def check_sources(**context):
    from clickhouse_driver import Client as CHClient
    import psycopg2

    ch = CHClient(host=CH_HOST, port=CH_PORT)
    result = ch.execute("SELECT 1")
    assert result == [(1,)], "ClickHouse health check failed"

    count = ch.execute("SELECT count() FROM emg_sensor_data")
    sensor_rows = count[0][0]
    assert sensor_rows > 0, "emg_sensor_data is empty"

    conn = psycopg2.connect(
        host=CRM_HOST, port=CRM_PORT,
        dbname=CRM_DB, user=CRM_USER, password=CRM_PASSWORD
    )
    cur = conn.cursor()
    cur.execute("SELECT count(*) FROM customers")
    crm_rows = cur.fetchone()[0]
    assert crm_rows > 0, "CRM customers table is empty"
    cur.close()
    conn.close()

    context['ti'].xcom_push(key='sensor_rows', value=sensor_rows)
    context['ti'].xcom_push(key='crm_rows', value=crm_rows)
    print(f"Sources OK: emg_sensor_data={sensor_rows} rows, customers={crm_rows} rows")


def truncate_view(**context):
    from clickhouse_driver import Client as CHClient

    ch = CHClient(host=CH_HOST, port=CH_PORT)
    ch.execute("TRUNCATE TABLE IF EXISTS user_reports")
    print("user_reports mart cleared")


def build_report_view(**context):
    from clickhouse_driver import Client as CHClient

    ch = CHClient(host=CH_HOST, port=CH_PORT)

    query = f"""
        INSERT INTO user_reports (
            user_id,
            customer_name,
            customer_email,
            prosthesis,
            total_signals,
            min_signal_time,
            max_signal_time,
            report_updated
        )
        SELECT
            e.user_id                       AS user_id,
            coalesce(c.name, 'Unknown')     AS customer_name,
            coalesce(c.email, '')           AS customer_email,
            e.prosthesis.                   AS prosthesis,
            count()                         AS total_signals,
            min(e.signal_time)              AS min_signal_time,
            max(e.signal_time)              AS max_signal_time,
            now()                           AS report_updated
        FROM emg_sensor_data AS e
        LEFT JOIN postgresql(
            '{CRM_HOST}:{CRM_PORT}',
            '{CRM_DB}',
            'customers',
            '{CRM_USER}',
            '{CRM_PASSWORD}'
        ) AS c ON e.user_id = toUInt32(c.id)
        GROUP BY
            e.user_id,
            c.name,
            c.email,
            e.prosthesis
        ORDER BY
            e.user_id,
            e.prosthesis
    """

    ch.execute(query)
    print("user_reports mart prepared")


def verify_view(**context):
    from clickhouse_driver import Client as CHClient

    ch = CHClient(host=CH_HOST, port=CH_PORT)
    total = ch.execute("SELECT count() FROM user_reports")[0][0]
    assert total > 0, "user_reports mart is empty"

    users = ch.execute("SELECT uniq(user_id) FROM user_reports")[0][0]

    dates = ch.execute("""
        SELECT
            min(min_signal_time),
            max(max_signal_time)
        FROM user_reports
    """)[0]

    sample = ch.execute("""
        SELECT user_id, customer_name, prosthesis,
               total_signals
        FROM user_reports
        ORDER BY total_signals DESC
        LIMIT 3
    """)

    context['ti'].xcom_push(key='view_rows', value=total)
    context['ti'].xcom_push(key='view_users', value=users)

default_args = {
    'owner': 'bionicpro',
    'retries': 2,
    'retry_delay': timedelta(minutes=5),
}

with DAG(
    dag_id='etl_reports',
    default_args=default_args,
    schedule_interval='@daily',
    start_date=datetime(2025, 1, 1),
    catchup=False,
    tags=['etl', 'reports', 'clickhouse'],
) as dag:

    t_check = PythonOperator(
        task_id='check_sources',
        python_callable=check_sources,
    )

    t_truncate = PythonOperator(
        task_id='truncate_view',
        python_callable=truncate_view,
    )

    t_build = PythonOperator(
        task_id='build_report_view',
        python_callable=build_report_view,
    )

    t_verify = PythonOperator(
        task_id='verify_view',
        python_callable=verify_view,
    )

    t_check >> t_truncate >> t_build >> t_verify