{%- if include.object %}
{%- assign properties = include.object.properties %}
{%- elsif include.properties %}
{%- assign properties = include.properties %}
{%- endif %}
<ul>
{% for property in properties %}
  {% assign propertyName = property[0] %}
  {% assign propertyData = property[1] %}
  {% capture defaultValue %}{{ propertyData.default | to_string  }}{% endcapture %}
  <li><code class="highlighter-rouge">{{ propertyName }}</code>
  {%- if propertyData.type %} <i>(
	{%- case propertyData.type %}
	{%- when "array" %}массив
	{%- when "object" %}объект
	{%- when "integer" %}целочисленный
	{%- when "string" %}строка
	{%- when "boolean" %}булевый
	{%- else %}{{ propertyData.type }}
	{%- endcase -%}
	{%- if propertyData.items.type %}
	  {%- case propertyData.items.type %}
	  {%- when "array" %} массивов
	  {%- when "object" %} объектов
  	  {%- when "integer" %} целых чисел
	  {%- when "string" %} строк
	  {%- when "boolean" %} булевых значений
	  {%- else %} из {{ propertyData.type }}
	  {%- endcase -%}
	{%- endif -%}
	)</i>
  {%- endif %}
  {%- if propertyData.enum %}
	<p>Допустимые значения: <code class="highlighter-rouge">{{ propertyData.enum | join: "</code>, <code class='highlighter-rouge'>" }}</code></p>
  {%- endif %}
  {%- if defaultValue.size > 0 %}
	{%- if defaultValue != '{}' > 0 %}
	  <p>По умолчанию: <code class="highlighter-rouge">{{ defaultValue }}</code>.</p>
	{%- endif %}
  {%- endif %}
  {{- propertyData.description | markdownify }}
  {%- if propertyData.example %}
	<p>Пример: <code class="highlighter-rouge">{{ propertyData.example }}</code></p>
  {%- endif %}
  {%- if propertyData.properties %}
	{% include jsonschema_object.md properties=propertyData.properties %}
  {%- endif %}
  {% for item in propertyData.items %}
	{%- if item[0] == 'properties' %}
	  {%- assign itemProperties = item[1] %}
	  {% include jsonschema_object.md properties=itemProperties %}
	{%- endif %}
  {%- endfor %}
  </li>
{%- endfor %}
</ul>
