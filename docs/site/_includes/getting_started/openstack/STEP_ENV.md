You need to create a service account so that Deckhouse Platform can manage resources in the {{ page.platform_name[page.lang] }}. The detailed instructions for creating a service account are available in the [provider's documentation](https://docs.openstack.org/keystone/pike/admin/cli-keystone-manage-services.html). Below is a brief sequence of actions necessary to obtain authorization data (we use [Mail.ru Cloud Solutions](https://mcs.mail.ru/) cloud services as an example):
- Follow this [link](https://mcs.mail.ru/app/project/keys/);
- Switch to the «API keys» tab;
- Click the «Download openrc version 3» button;
- Run the downloaded shell script. It will create values for environment variables to use in the `provider` parameters of the Deckhouse Platform configuration.
