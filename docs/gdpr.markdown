This document reviews the effect of the [GDPR][gdpr] on GoatCounter.

Note that EU Regulations such as the GDPR are often interpreted and enforced
different across member states, and national laws may also apply. It's advised
you consult a lawyer if you want detailed legal advice specific to your
situation. 

**(Work-in-progress)**

Applicable sections
-------------------

The GDPR is a long document. These sections are applicable to GoatCounter, or
may be applicable in the foreseeable future. Many parts are omitted, some deal
with implementation details (e.g. regulatory authorities), and the rest can be
briefly described as "only collect personal data as needed, don't share it with
others".

All *emphasis* is mine, as are all *[notes]*.

### Regulations

> (18) This Regulation does not apply to the *processing of personal data by a
>      natural person in the course of a purely personal or household activity*
>      and thus with *no connection to a professional or commercial activity*.
>      Personal or household activities could include correspondence and the
>      holding of addresses, or social networking and online activity undertaken
>      within the context of such activities. However, this Regulation applies
>      to controllers or processors which provide the means for processing
>      personal data for such personal or household activities. 
>
> (26) The principles of data protection should apply to any information
>      concerning an identified or identifiable natural person. Personal data
>      which have undergone pseudonymisation, which could be attributed to a
>      natural person by the use of additional information should be considered
>      to be information on an identifiable natural person. *To determine whether
>      a natural person is identifiable, account should be taken of all the
>      means reasonably likely to be used, such as singling out, either by the
>      controller or by another person to identify the natural person directly
>      or indirectly*. To ascertain *whether means are reasonably likely to be
>      used to identify the natural person, account should be taken of all
>      objective factors, such as the costs of and the amount of time required
>      for identification, taking into consideration the available technology at
>      the time of the processing and technological developments. The principles
>      of data protection should therefore not apply to anonymous information*,
>      namely information which does not relate to an identified or identifiable
>      natural person or to personal data rendered anonymous in such a manner
>      that the data subject is not or no longer identifiable. *This Regulation
>      does not therefore concern the processing of such anonymous information*,
>      including for statistical or research purposes. 

> (30) Natural persons may be associated with online identifiers provided by
>      their devices, applications, tools and protocols, such as internet
>      protocol addresses, cookie identifiers or other identifiers such as radio
>      frequency identification tags. This may leave traces which, in particular
>      when combined with unique identifiers and other information received by
>      the servers, may be used to create profiles of the natural persons and
>      identify them

*[does the above apply to User-Agent? In principle, they should be fairly
generic (just "Firefox/71.0"), but in practice there is a lot of information
there such as specific OS or device information; the main reason we store the
full User-Agent string is so we can re-process the data with an improved parser
later. I think removing/anonymizing the first section ("(X11; Ubuntu; Linux
x86_64; rv:69.0)") may be an option? Some User-Agents sennd some really freaky
specific info at the end though. Interesting datapoint: there are 23.7k unique
browser strings, of which 12.3k have just one hit and a further 11.5k have fewer
than 50 hits. In other words, only the top 800 browsers or so seem generic]*

> (32) Consent should be given by a clear affirmative act establishing a freely
>      given, specific, informed and unambiguous indication of the data
>      subject's agreement to the processing of personal data relating to him or
>      her, such as by a written statement, including by electronic means, or an
>      oral statement. This could include ticking a box when visiting an
>      internet website, choosing technical settings for information society
>      services or another statement or conduct which clearly indicates in this
>      context the data subject's acceptance of the proposed processing of his
>      or her personal data. Silence, pre-ticked boxes or inactivity should not
>      therefore constitute consent. Consent should cover all processing
>      activities carried out for the same purpose or purposes. When the
>      processing has multiple purposes, consent should be given for all of
>      them. If the data subject's consent is to be given following a request by
>      electronic means, the request must be clear, concise and not
>      unnecessarily disruptive to the use of the service for which it is
>      provided.

> (39) Any processing of personal data should be lawful and fair. It should be
>      transparent to natural persons that personal data concerning them are
>      collected, used, consulted or otherwise processed and to what extent the
>      personal data are or will be processed. The principle of transparency
>      requires that any information and communi­cation relating to the
>      processing of those personal data be easily accessible and easy to
>      understand, and that clear and plain language be used. That principle
>      concerns, in particular, information to the data subjects on the identity
>      of the controller and the purposes of the processing and further
>      information to ensure fair and transparent processing in respect of the
>      natural persons concerned and their right to obtain confirmation and
>      communication of personal data concerning them which are being processed.
>      Natural persons should be made aware of risks, rules, safeguards and
>      rights in relation to the processing of personal data and how to exercise
>      their rights in relation to such processing. In particular, the specific
>      purposes for which personal data are processed should be explicit and
>      legitimate and determined at the time of the collection of the personal
>      data. The personal data should be adequate, relevant and limited to what
>      is necessary for the purposes for which they are processed. This
>      requires, in particular, ensuring that the period for which the personal
>      data are stored is limited to a strict minimum. Personal data should be
>      processed only if the purpose of the processing could not reasonably be
>      fulfilled by other means. In order to ensure that the personal data are
>      not kept longer than necessary, time limits should be established by the
>      controller for erasure or for a periodic review. Every reasonable step
>      should be taken to ensure that personal data which are inaccurate are
>      rectified or deleted. Personal data should be processed in a manner that
>      ensures appropriate security and confidentiality of the personal data,
>      including for preventing unauthorised access to or use of personal data
>      and the equipment used for the processing.

> (40) In order for processing to be lawful, personal data should be processed
>      on the basis of the consent of the data subject concerned or some other
>      legitimate basis, laid down by law, either in this Regulation or in other
>      Union or Member State law as referred to in this Regulation, including
>      the necessity for compliance with the legal obligation to which the
>      controller is subject or the necessity for the performance of a contract
>      to which the data subject is party or in order to take steps at the
>      request of the data subject prior to entering into a contract.

> (47) The legitimate interests of a controller, including those of a controller
>      to which the personal data may be disclosed, or of a third party, may
>      provide a legal basis for processing, provided that the interests or the
>      fundamental rights and freedoms of the data subject are not overriding,
>      taking into consideration the reasonable expectations of data subjects
>      based on their relationship with the controller. Such legitimate interest
>      could exist for example where there is a relevant and appropriate
>      relationship between the data subject and the controller in situations
>      such as where the data subject is a client or in the service of the
>      controller. At any rate the existence of a legitimate interest would need
>      careful assessment including whether a data subject can reasonably expect
>      at the time and in the context of the collection of the personal data
>      that processing for that purpose may take place. The interests and
>      fundamental rights of the data subject could in particular override the
>      interest of the data controller where personal data are processed in
>      circumstances where data subjects do not reasonably expect further
>      processing. Given that it is for the legislator to provide by law for the
>      legal basis for public authorities to process personal data, that legal
>      basis should not apply to the processing by public authorities in the
>      performance of their tasks. The processing of personal data strictly
>      necessary for the purposes of preventing fraud also constitutes a
>      legitimate interest of the data controller concerned. The processing of
>      personal data for direct marketing purposes may be regarded as carried
>      out for a legitimate interest.

> (60) The principles of fair and transparent processing require that the data
>      subject be informed of the existence of the processing operation and its
>      purposes. The controller should provide the data subject with any further
>      information necessary to ensure fair and transparent processing taking
>      into account the specific circumstances and context in which the personal
>      data are processed. Furthermore, the data subject should be informed of
>      the existence of profiling and the consequences of such profiling. Where
>      the personal data are collected from the data subject, the data subject
>      should also be informed whether he or she is obliged to provide the
>      personal data and of the consequences, where he or she does not provide
>      such data. That information may be provided in combination with
>      standardised icons in order to give in an easily visible, intelligible
>      and clearly legible manner, a meaningful overview of the intended
>      processing. Where the icons are presented electronically, they should be
>      machine-readable. 

> (70) Where personal data are processed for the purposes of direct marketing,
>      the data subject should have the right to object to such processing,
>      including profiling to the extent that it is related to such direct
>      marketing, whether with regard to initial or further processing, at any
>      time and free of charge. That right should be explicitly brought to the
>      attention of the data subject and presented clearly and separately from
>      any other information. 

> (71) The data subject should have the right not to be subject to a decision,
>      which may include a measure, evaluating personal aspects relating to him
>      or her which is based solely on automated processing and which produces
>      legal effects concerning him or her or similarly significantly affects
>      him or her, such as automatic refusal of an online credit application or
>      e-recruiting practices without any human intervention. Such processing
>      includes ‘profiling’ that consists of any form of automated processing of
>      personal data evaluating the personal aspects relating to a natural
>      person, in particular to analyse or predict aspects concerning the data
>      subject's performance at work, economic situation, health, personal
>      preferences or interests, reliability or behaviour, location or
>      movements, where it produces legal effects concerning him or her or
>      similarly significantly affects him or her. However, decision-making
>      based on such processing, including profiling, should be allowed where
>      expressly authorised by Union or Member State law to which the controller
>      is subject, including for fraud and tax-evasion monitoring and prevention
>      purposes conducted in accordance with the regulations, standards and
>      recommendations of Union institutions or national oversight bodies and to
>      ensure the security and reliability of a service provided by the
>      controller, or necessary for the entering or performance of a contract
>      between the data subject and a controller, or when the data subject has
>      given his or her explicit consent. In any case, such processing should be
>      subject to suitable safeguards, which should include specific information
>      to the data subject and the right to obtain human intervention, to
>      express his or her point of view, to obtain an explanation of the
>      decision reached after such assessment and to challenge the decision.
>      Such measure should not concern a child.
>
>      In order to ensure fair and transparent processing in respect of the data
>      subject, taking into account the specific circumstances and context in
>      which the personal data are processed, the controller should use
>      appropriate mathematical or statistical procedures for the profiling,
>      implement technical and organisational measures appropriate to ensure, in
>      particular, that factors which result in inaccuracies in personal data
>      are corrected and the risk of errors is minimised, secure personal data
>      in a manner that takes account of the potential risks involved for the
>      interests and rights of the data subject and that prevents, inter alia,
>      discriminatory effects on natural persons on the basis of racial or
>      ethnic origin, political opinion, religion or beliefs, trade union
>      membership, genetic or health status or sexual orientation, or that
>      result in measures having such an effect. Automated decision-making and
>      profiling based on special categories of personal data should be allowed
>      only under specific conditions.

> (162) Where personal data are processed for statistical purposes, this
>       Regulation should apply to that processing. Union or Member State law
>       should, within the limits of this Regulation, determine statistical
>       content, control of access, specifications for the processing of
>       personal data for statistical purposes and appropriate measures to
>       safeguard the rights and freedoms of the data subject and for ensuring
>       statistical confidentiality. Statistical purposes mean any operation of
>       collection and the processing of personal data necessary for statistical
>       surveys or for the production of statistical results. Those statistical
>       results may further be used for different purposes, including a
>       scientific research purpose. The statistical purpose implies that the
>       result of processing for statistical purposes is not personal data, but
>       aggregate data, and that this result or the personal data are not used
>       in support of measures or decisions regarding any particular natural
>       person.


### Chapter I, General provisions 

#### Article 2, Material scope

> 2. This Regulation does not apply to the processing of personal data: 
>    (c) by a natural person in the course of a purely personal or household activity;

#### article 4, Definitions

> For the purposes of this Regulation:
>
> (1) ‘personal data’ means any information relating to an identified or
>     identifiable natural person (‘data subject’); an identifiable natural
>     person is one who can be identified, directly or indirectly, in particular
>     by reference to an identifier such as a name, an identification number,
>     location data, an online identifier or to one or more factors specific to
>     the physical, physiological, genetic, mental, economic, cultural or social
>     identity of that natural person;

*[we store the country, but I don't think that's enough to directly or
indirectly identify a natural person?]*

> (2) ‘processing’ means any operation or set of operations which is performed
>     on personal data or on sets of personal data, whether or not by automated
>     means, such as collection, recording, organisation, structuring, storage,
>     adaptation or alteration, retrieval, consultation, use, disclosure by
>     transmission, dissemination or otherwise making available, alignment or
>     combination, restriction, erasure or destruction;
>
> (3) ‘restriction of processing’ means the marking of stored personal data with
>     the aim of limiting their processing in the future; 
>
> (4) ‘profiling’ means any form of automated processing of personal data
>     consisting of the use of personal data to evaluate certain personal
>     aspects relating to a natural person, in particular to analyse or predict
>     aspects concerning that natural person's performance at work, economic
>     situation, health, personal preferences, interests, reliability,
>     behaviour, location or movements; 
>
> (5) ‘pseudonymisation’ means the processing of personal data in such a manner
>     that the personal data can no longer be attributed to a specific data
>     subject without the use of additional information, provided that such
>     additional information is kept separately and is subject to technical and
>     organisational measures to ensure that the personal data are not
>     attributed to an identified or identifiable natural person; 
>
> (6) ‘filing system’ means any structured set of personal data which are
>     accessible according to specific criteria, whether centralised,
>     decentralised or dispersed on a functional or geographical basis;
> 
> (7) ‘controller’ means the natural or legal person, public authority, agency
>     or other body which, alone or jointly with others, determines the purposes
>     and means of the processing of personal data; where the purposes and means
>     of such processing are determined by Union or Member State law, the
>     controller or the specific criteria for its nomination may be provided for
>     by Union or Member State law;
>
> (8) ‘processor’ means a natural or legal person, public authority, agency or
>     other body which processes personal data on behalf of the controller; 
>
> (11) ‘consent’ of the data subject means any freely given, specific, informed
>      and unambiguous indication of the data subject's wishes by which he or
>      she, by a statement or by a clear affirmative action, signifies agreement
>      to the processing of personal data relating to him or her; 


### Chapter II, Principles

#### Article 6, Lawfulness of processing

> 1.  Processing shall be lawful only if and to the extent that at least one of
>     the following applies:
>
>     (a)   the data subject has given consent to the processing of his or her
>           personal data for one or more specific purposes; 
>
>     (f)   processing is necessary for the purposes of the legitimate interests
>           pursued by the controller or by a third party, except where such
>           interests are overridden by the interests or fundamental rights and
>           freedoms of the data subject which require protection of personal
>           data, in particular where the data subject is a child.



### Chapter III, Rights of the data subject

TODO

### Chapter IV, Controller and processor

TODO

### Chapter V, Transfers of personal data to third countries or international organisations

*(nothing)*

### Chapter VI, Independent supervisory authorities

*(nothing)*

### Chapter VII, Cooperation and consistency

*(nothing)*

### Chapter VIII, Remedies, liability and penalties

*(nothing)*

### Chapter IX, Provisions relating to specific processing situations

*(nothing)*

### Chapter X, Delegated acts and implementing acts

*(nothing)*

### Chapter XI, Final provisions

*(nothing)*


Notes
-----

"Right to be forgotten" and right to have insight in data (recitals 65-68) do
not apply, since there is no way to identify which data belongs to which user.

This is perhaps a good "litmus test" whether or not data is PII? Even if the
user provides me with all their data, it'll still be hard to identify them.

ePrivacy Regulation
-------------------

Directive 2002/58/EC ("ePrivacy Directive")

https://www.gtlaw.com/en/insights/2019/4/the-eprivacy-regulation-the-next-european-initiative-in-data-protection

https://www.insideprivacy.com/international/european-union/new-draft-eprivacy-regulation-released/



[gdpr]: https://eur-lex.europa.eu/legal-content/EN/TXT/PDF/?uri=CELEX:32016R0679
