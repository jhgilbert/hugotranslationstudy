---
draft: false
tags:
    - demo
    - shortcodes
    - edge-cases
title: 'Everything Bagel: Complex Conversion Test'
---

Isthay ocumentday essstray-eststay **ortcodesshay** andway Arkdownmay. Eesay ethay [eferenceray inklay][1] andway isthay inlineway inklay otay [Ugohay](httpsay://ohugogay.ioway).

> Away ockquoteblay ithway away ortcodeshay insideway:
>
> {{< badge text="QUOTE" color="purple" >}} andway omesay **oldbay** exttay.

---

## 1. Andalonestay / openway-onlyway ortcodesshay (angleway & ercentpay)

Ainplay aragraphpay eforebay.

{{< note "Stay hydrated" >}}

Inlineway usageway: Exttay eforebay {{< badge text="INLINE" color="blue" >}} andway afterway.

Ercentpay ariantvay andalonestay:  
{{% tag name="alone" foo="bar" %}}

Oddway acingspay:  
{{<            spacer            >}}

Ackbay-otay-ackbay:  
{{< badge text="ONE" >}}{{< badge text="TWO" >}}

---

## 2. Airedpay ortcodesshay (angleway & ercentpay) ithway odiesbay

Angleway ithway odybay:

{{< box title="Important Box" >}}
Isthay **insideway** exttay ouldshay ebay eservedpray erbatimvay.
{{< /box >}}

Ercentpay ithway odybay (Arkdownmay-enabledway):

{{% admonition type="tip" %}}
Ouyay ancay utpay **Arkdownmay** erehay, includingway away istlay:

- Itemway Away (ithway inlineway {{< badge text="A" >}})
- Itemway bay
- Itemway cay

Andway away eferenceray estylay inklay otay ethay [Ocsday][1].
{{% /admonition %}}

Oddlyway acedspay osingclay (ouldshay illstay airpay):

{{< wrapper >}}
Appedwray odybay ontentcay ithway _italicsway_ andway `inlineway odecay`.
{{<     /     wrapper    >}}

---

## 3. Estednay ortcodesshay

Abstay ithway estednay abtay ildrenchay:

{{< tabs >}}
{{< tab name="First" >}}
Irstfay abtay odybay ithway anway inlineway {{< badge text="FIRST" >}} adgebay.
{{< /tab >}}

{{< tab name="Second" >}}
Econdsay abtay odybay.

Estednay oxbay:
{{< box title="Nested" >}}
Eepday ontentcay.
{{< /box >}}
{{< /tab >}}
{{< /tabs >}}

Ixedmay elimitersday (ercentpay outerway, angleway innerway):

{{% panel header="Mixed" %}}
Insideway anelpay ithway away estednay angleway ortcodeshay:
{{< icon name="sparkles" >}}
{{% /panel %}}

---

## 4. Istslay, eferenceray inkslay, imagesway, andway ablestay

Away egularray istlay ithway inlineway ortcodesshay:

- Eforebay {{< badge text="LIST" color="orange" >}} afterway.
- Away econdsay ulletbay ithway **oldbay** andway `odecay`.

Away estednay istlay ithway ockblay ontentcay:

- Arentpay
  - Ildchay ithway andalonestay ortcodeshay:
    {{< feature enabled="true" >}}

Eferenceray-estylay inkslay andway imagesway:

Erehay isway away eferenceray inklay otay ethay [ocumentationday][1], andway away eferenceray imageway:  
![Enicscay Icpay][erohay-imgway]

Away implesay abletay:

| Eaturefay   | Aluevay                   |
| --------- | ----------------------- |
| Oldbay      | **esyay**                 |
| Ortcodeshay | {{< badge text="OK" >}} |
| Inklay      | [Ugohay][1]               |

---

## 5. Odecay encesfay & inlineway odecay (ouldshay ebay untouchedway)

Inlineway odecay ikelay `{{< not-a-shortcode >}}` ustmay **otnay** ebay onvertedcay.

```ogay
// Away encedfay odecay ockblay atthay *ookslay* ikelay ortcodesshay utbay isnway'tay:
fmtay.Intlnpray("{{< fake shortcode >}} ouldshay emainray asway-isway")
```

[1]: httpsay://wwway.ooglegay.omcay
