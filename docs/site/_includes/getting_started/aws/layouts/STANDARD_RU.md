![resources](https://docs.google.com/drawings/d/e/2PACX-1vSkzOWvLzAwB4hmIk4CP1-mj2QIxCyJg2VJvijFfdttjnV0quLpw7x87KtTC5v2I9xF5gVKpTK-aqyz/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1kln-DJGFldcr6gayVtFYn_3S50HFIO1PLTc1pC_b3L0/edit --->

**Важно!** Возможность использования публичных IP временно отозвана в связи с тем, что "публичные" инстансы не получают маршруты к подам на "серых" инстансах.

В данной схеме размещения виртуальные машины будут выходить в интернет через NAT Gateway с общим и единственным source IP. Все узлы, созданные с помощью dhctl, опционально могут получить публичный IP (ElasticIP).
